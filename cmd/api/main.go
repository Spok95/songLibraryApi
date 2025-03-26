package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
	"log"
	"net/http"
	"net/url"
	"os"
	"songLibraryApi/internal/models"
	"strconv"
	"strings"
)

type Song struct {
	ID    int            `json:"id"`
	Group string         `json:"group"`
	Title string         `json:"song"`
	Text  sql.NullString `json:"text,omitempty"`
	Link  sql.NullString `json:"link,omitempty"`
	Date  sql.NullString `json:"releaseDate,omitempty"`
}

type SongResponse struct {
	ID    int    `json:"id"`
	Group string `json:"group"`
	Title string `json:"song"`
	Text  string `json:"text,omitempty"`
	Link  string `json:"link,omitempty"`
	Date  string `json:"releaseDate,omitempty"`
}

func addSongHandler(w http.ResponseWriter, r *http.Request) {
	var song Song
	// Разбор JSON
	if err := json.NewDecoder(r.Body).Decode(&song); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Проверка обязательных полей
	if song.Group == "" || song.Title == "" {
		http.Error(w, "Missing 'group' or 'song'", http.StatusBadRequest)
		return
	}

	// Обогащение через внешний API
	detail, err := fetchSongInfo(song.Group, song.Title)
	if err != nil {
		log.Printf("Failed to enrich song: %v", err)
		http.Error(w, "Failed to fetch song details", http.StatusInternalServerError)
		return
	}

	// Заполняем song перед сохранением
	song.Text = sql.NullString{String: detail.Text, Valid: true}
	song.Link = sql.NullString{String: detail.Link, Valid: true}
	song.Date = sql.NullString{String: detail.ReleaseDate, Valid: true}

	// Вставка в БД
	query := `INSERT INTO songs (group_name, song_title, text, link, release_date) VALUES ($1, $2, $3, $4, $5) RETURNING id`
	err = db.QueryRow(query,
		song.Group,
		song.Title,
		detail.Text,
		detail.Link,
		detail.ReleaseDate,
	).Scan(&song.ID)
	if err != nil {
		log.Printf("DB insert error: %v", err)
		http.Error(w, "Failed to save song", http.StatusInternalServerError)
		return
	}

	log.Printf("✅ Сохранили песню: %s - %s (ID: %d)", song.Group, song.Title, song.ID)

	// Возврат клиенту
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(toResponse(song))
}

func getSongsHandler(w http.ResponseWriter, r *http.Request) {
	// Читаем query-параметры
	group := r.URL.Query().Get("group")
	song := r.URL.Query().Get("song")

	// Пагинация
	pageStr := r.URL.Query().Get("page")
	limitStr := r.URL.Query().Get("limit")

	// Значения по умолчанию
	page := 1
	limit := 10
	var err error

	if pageStr != "" {
		page, err = strconv.Atoi(pageStr)
		if err != nil || page < 1 {
			page = 1
		}
	}
	if limitStr != "" {
		limit, err = strconv.Atoi(limitStr)
		if err != nil || limit < 1 {
			limit = 10
		}
	}
	offset := (page - 1) * limit

	// Построение SQL-запроса динамически
	query := `SELECT id, group_name, song_title FROM songs`
	args := []interface{}{}
	conditions := []string{}

	if group != "" {
		conditions = append(conditions, "group_name ILIKE $"+strconv.Itoa(len(args)+1))
		args = append(args, "%"+group+"%")
	}
	if song != "" {
		conditions = append(conditions, "song_title ILIKE $"+strconv.Itoa(len(args)+1))
		args = append(args, "%"+song+"%")
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND")
	}

	query += fmt.Sprintf(" ORDER BY id DESC LIMIT $%d OFFSET $%d", len(args)+1, len(args)+2)
	args = append(args, limit, offset)

	// Выполняем запрос
	rows, err := db.Query(query, args...)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		log.Println("Query error:", err)
		return
	}
	defer rows.Close()

	var songs []Song
	for rows.Next() {
		var s Song
		if err := rows.Scan(&s.ID, &s.Group, &s.Title); err != nil {
			http.Error(w, "Row scan error", http.StatusInternalServerError)
			log.Println("Row scan error:", err)
			return
		}
		songs = append(songs, s)
	}

	// Отправляем JSON-ответ
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(songs)
}

func getSongByIDHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]

	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid song ID", http.StatusBadRequest)
		return
	}

	var s Song
	query := `SELECT id, group_name, song_title, text, link, release_date FROM songs WHERE id = $1`
	err = db.QueryRow(query, id).Scan(&s.ID, &s.Group, &s.Title, &s.Text, &s.Link, &s.Date)
	if err == sql.ErrNoRows {
		http.Error(w, "Song not found", http.StatusNotFound)
		return
	} else if err != nil {
		log.Printf("Database error: %v", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	response := SongResponse{
		ID:    s.ID,
		Group: s.Group,
		Title: s.Title,
		Text:  nullableToString(s.Text),
		Link:  nullableToString(s.Link),
		Date:  nullableToString(s.Date),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func nullableToString(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Welcome to Song Library API!")
}

func fetchSongInfo(group, song string) (*models.SongDetail, error) {
	// Заменить на реальный адрес API при проверке
	baseURL := "http://localhost:8081/info"

	// Создаём запрос вида /info?group=Muse&song=...
	reqURL := fmt.Sprintf("%s?group=%s&song=%s", baseURL, url.QueryEscape(group), url.QueryEscape(song))

	resp, err := http.Get(reqURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch song info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("external API returned status: %d", resp.StatusCode)
	}

	var detail models.SongDetail
	if err := json.NewDecoder(resp.Body).Decode(&detail); err != nil {
		return nil, fmt.Errorf("failed to decode JSON: %w", err)
	}
	return &detail, nil

	//log.Printf("⚠️ MOCK fetchSongInfo called for: %s - %s", group, song)
	//return &SongDetail{
	//	ReleaseDate: "16.07.2006",
	//	Text:        "Ooh baby, don't you know I suffer?...",
	//	Link:        "https://youtube.com/watch?v=Xsp3_a-PMTw",
	//}, nil
}

func toResponse(s Song) SongResponse {
	return SongResponse{
		ID:    s.ID,
		Group: s.Group,
		Title: s.Title,
		Text:  nullableToString(s.Text),
		Link:  nullableToString(s.Link),
		Date:  nullableToString(s.Date),
	}
}

func loadEnv() {
	if err := godotenv.Load(); err != nil {
		log.Fatal("Ошибка загрузки .env файла")
	}
}

var db *sql.DB

func initDB() {
	if err := godotenv.Load(); err != nil {
		log.Fatal("Error loading .env file")
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL not set")
	}

	var err error
	db, err = sql.Open("pgx", dsn)
	if err != nil {
		log.Fatalf("Error opening DB: %v", err)
	}

	if err := db.Ping(); err != nil {
		log.Fatalf("DB unreachable: %v", err)
	}

	log.Println("Connected to PostgreSQL!")
}

func migrate() {
	query := `
CREATE TABLE IF NOT EXISTS songs (
id SERIAL PRIMARY KEY,
group_name TEXT NOT NULL,
song_title TEXT NOT NULL,
text TEXT,
link TEXT,
release_date TEXT
                                 );
`
	if _, err := db.Exec(query); err != nil {
		log.Fatalf("Migration failed: %v", err)
	}
	log.Println("Migration complete.")
}

func main() {
	loadEnv()
	initDB()
	migrate()
	r := mux.NewRouter()
	r.HandleFunc("/", homeHandler).Methods("GET")
	r.HandleFunc("/songs", addSongHandler).Methods("POST")
	r.HandleFunc("/songs", getSongsHandler).Methods("GET")
	r.HandleFunc("/songs/{id}", getSongByIDHandler).Methods("GET")

	log.Println("Server is running on port 8080...")
	http.ListenAndServe(":8080", r)
}
