// @title           Song Library API
// @version         1.0
// @description     API для управления онлайн-библиотекой песен
// @termsOfService  http://swagger.io/terms/

// @contact.name   Konstantin
// @contact.url    https://github.com/Spok95
// @contact.email  kostya2211@yandex.ru

// @license.name  MIT
// @license.url   https://opensource.org/licenses/MIT

// @host      localhost:8080
// @BasePath  /

package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
	"github.com/swaggo/http-swagger"
	"log"
	"net/http"
	"net/url"
	"os"
	_ "songLibraryApi/docs"
	"songLibraryApi/internal/models"
	"strconv"
	"strings"
)

// AddSongHandler добавляет новую песню
// @Summary      Добавить песню
// @Description  Создаёт новую песню и обогащает её данными
// @Tags         Songs
// @Accept       json
// @Produce      json
// @Param        song  body  models.Song  true  "Данные песни"
// @Success      201   {object}  models.SongResponse
// @Failure      400   {string}  string  "Invalid input"
// @Router       /songs [post]
func addSongHandler(w http.ResponseWriter, r *http.Request) {
	var song models.Song
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
	json.NewEncoder(w).Encode(models.ToResponse(song))
}

// GetSongsHandler возвращает список всех песен
// @Summary      Получить список песен
// @Description  Возвращает список всех песен с фильтрацией и пагинацией
// @Tags         Songs
// @Produce      json
// @Success      200  {array}  models.SongResponse
// @Router       /songs [get]
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
	query := `SELECT id, group_name, song_title, text, link, release_date FROM songs`
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

	var songs []models.Song
	for rows.Next() {
		var s models.Song
		if err := rows.Scan(&s.ID, &s.Group, &s.Title, &s.Text, &s.Link, &s.Date); err != nil {
			http.Error(w, "Row scan error", http.StatusInternalServerError)
			log.Println("Row scan error:", err)
			return
		}
		songs = append(songs, s)
	}

	var response []models.SongResponse
	for _, s := range songs {
		response = append(response, models.ToResponse(s))
	}

	// Отправляем JSON-ответ
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetSongHandler возвращает информацию о песне по ID
// @Summary      Получить песню
// @Description  Возвращает данные о конкретной песне по её ID
// @Tags         Songs
// @Produce      json
// @Param        id   path      int  true  "ID песни"
// @Success      200  {object}  models.SongResponse
// @Failure      404  {string}  string  "Song not found"
// @Router       /songs/{id} [get]
func getSongByIDHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]

	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid song ID", http.StatusBadRequest)
		return
	}

	var s models.Song
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

	response := models.SongResponse{
		ID:    s.ID,
		Group: s.Group,
		Title: s.Title,
		Text:  models.NullableToString(s.Text),
		Link:  models.NullableToString(s.Link),
		Date:  models.NullableToString(s.Date),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// UpdateSongHandler обновляет данные песни
// @Summary      Обновить песню
// @Description  Изменяет информацию о песне по ID
// @Tags         Songs
// @Accept       json
// @Produce      json
// @Param        id    path  int         true  "ID песни"
// @Param        song  body  models.Song  true  "Обновлённые данные"
// @Success      200   {object}  models.SongResponse
// @Failure      400   {string}  string  "Invalid input"
// @Failure      404   {string}  string  "Song not found"
// @Router       /songs/{id} [put]
func updateSongHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]

	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid song ID", http.StatusBadRequest)
		return
	}

	var updated models.Song
	if err := json.NewDecoder(r.Body).Decode(&updated); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	query := `UPDATE songs
SET group_name = $1, song_title = $2
WHERE id = $3`

	_, err = db.Exec(query, updated.Group, updated.Title, id)
	if err != nil {
		log.Printf("Update error: %v", err)
		http.Error(w, "Failed to update song", http.StatusInternalServerError)
		return
	}
	log.Printf("📝 Обновлена песня ID %d", id)
	w.WriteHeader(http.StatusNoContent)
}

// DeleteSongHandler удаляет песню по ID
// @Summary      Удалить песню
// @Description  Удаляет песню из базы данных
// @Tags         Songs
// @Param        id  path  int  true  "ID песни"
// @Success      204  "No Content"
// @Failure      404  {string}  string  "Song not found"
// @Router       /songs/{id} [delete]
func deleteSongHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]

	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid song ID", http.StatusBadRequest)
		return
	}
	query := `DELETE FROM songs WHERE id = $1`
	_, err = db.Exec(query, id)
	if err != nil {
		log.Printf("Delete error: %v", err)
		http.Error(w, "Failed to delete song", http.StatusInternalServerError)
		return
	}
	log.Printf("❌ Удалена песня ID %d", id)
	w.WriteHeader(http.StatusNoContent)
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Welcome to Song Library API!")
}

func fetchSongInfo(group, song string) (models.SongDetail, error) {
	// Заменить на реальный адрес API при проверке
	mockAPIURL := os.Getenv("MOCK_API_URL")
	if mockAPIURL == "" {
		mockAPIURL = "http://mockapi:8081"
	}

	// Создаём запрос вида /info?group=Muse&song=...
	reqURL := fmt.Sprintf("%s/info?group=%s&song=%s", mockAPIURL, url.QueryEscape(group), url.QueryEscape(song))
	log.Printf("🔎 Запрашиваем: %s", reqURL)

	resp, err := http.Get(reqURL)
	if err != nil {
		return models.SongDetail{}, fmt.Errorf("failed to fetch song info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return models.SongDetail{}, fmt.Errorf("external API returned status: %d", resp.StatusCode)
	}

	var detail models.SongDetail
	if err := json.NewDecoder(resp.Body).Decode(&detail); err != nil {
		return models.SongDetail{}, fmt.Errorf("failed to decode JSON: %w", err)
	}
	return detail, nil
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
	r.HandleFunc("/songs/{id}", updateSongHandler).Methods("PUT")
	r.HandleFunc("/songs/{id}", deleteSongHandler).Methods("DELETE")
	r.PathPrefix("/swagger/").Handler(httpSwagger.WrapHandler)

	log.Println("Server is running on port 8080...")
	http.ListenAndServe(":8080", r)
}
