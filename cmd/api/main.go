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
	"github.com/swaggo/http-swagger"
	"log"
	"net/http"
	"net/url"
	"os"
	_ "songLibraryApi/docs"
	"songLibraryApi/internal/models"
	"songLibraryApi/pkg/config"
	"strconv"
	"strings"
)

// Глобальная переменная для БД
var db *sql.DB

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

// @Summary Получить список песен
// @Description Возвращает список всех песен с фильтрацией и пагинацией
// @Tags Songs
// @Accept json
// @Produce json
// @Param group query string false "Фильтр по группе"
// @Param song query string false "Фильтр по названию песни"
// @Param releaseDate query string false "Фильтр по дате выпуска"
// @Param limit query int false "Количество песен на странице" default(10)
// @Param offset query int false "Смещение (для пагинации)" default(0)
// @Success 200 {array} models.SongResponse
// @Failure 500 {string} string "Ошибка сервера"
// @Router /songs [get]
func getSongsHandler(w http.ResponseWriter, r *http.Request) {
	query := `
SELECT id, group_name, song_title, text, link, release_date
FROM songs
WHERE ($1::TEXT IS NULL OR group_name = $1::TEXT)
AND ($2::TEXT IS NULL OR song_title = $2::TEXT)
AND ($3::TEXT IS NULL OR release_date = $3::TEXT)
LIMIT $4 OFFSET $5;`

	// Читаем параметры из URL
	group := r.URL.Query().Get("group")
	song := r.URL.Query().Get("song")
	releaseDate := r.URL.Query().Get("releaseDate")
	limitParam := r.URL.Query().Get("limit")
	offsetParam := r.URL.Query().Get("offset")

	// Значения по умолчанию
	offset := 0
	limit := 10

	if limitParam != "" {
		if parsedLimit, err := strconv.Atoi(limitParam); err != nil || parsedLimit <= 0 {
			http.Error(w, "Некорректное значение limit", http.StatusBadRequest)
			return
		} else {
			limit = parsedLimit
		}
	}
	if offsetParam != "" {

		if parsedOffset, err := strconv.Atoi(offsetParam); err != nil || parsedOffset < 0 {
			http.Error(w, "Некорректное значение offset", http.StatusBadRequest)
			return
		} else {
			offset = parsedOffset
		}
	}

	rows, err := db.Query(query, sql.NullString{String: group, Valid: group != ""},
		sql.NullString{String: song, Valid: song != ""},
		sql.NullString{String: releaseDate, Valid: releaseDate != ""},
		limit, offset)
	if err != nil {
		log.Printf("Ошибка запроса к БД: %v\nSQL: %s\nParams: group=%s, song=%s, releaseDate=%s, limit=%d, offset=%d",
			err, query, group, song, releaseDate, limit, offset)
		http.Error(w, "Ошибка сервера", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var songs []models.SongResponse
	for rows.Next() {
		var s models.Song
		err := rows.Scan(&s.ID, &s.Group, &s.Title, &s.Text, &s.Link, &s.Date)
		if err != nil {
			log.Printf("Ошибка чтения результата: %v", err)
			http.Error(w, "Ошибка сервера", http.StatusInternalServerError)
			return
		}
		songs = append(songs, models.ToResponse(s))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(songs)
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

	// Проверяем, что ID - это число
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid song ID", http.StatusBadRequest)
		return
	}

	// Проверяем, существует ли песня в БД
	var exists bool
	err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM songs WHERE id=$1)", id).Scan(&exists)
	if err != nil {
		log.Printf("Ошибка проверки существования песни: %v", err)
		http.Error(w, "Ошибка сервера", http.StatusInternalServerError)
		return
	}

	if !exists {
		log.Printf("❌ Песня ID %d не найдена, обновление не выполнено", id)
		http.Error(w, "Песня не найдена", http.StatusNotFound)
		return
	}

	// Читаем тело запроса
	var updated models.Song
	if err := json.NewDecoder(r.Body).Decode(&updated); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	query := `UPDATE songs SET group_name = $1, song_title = $2 WHERE id = $3`
	result, err := db.Exec(query, updated.Group, updated.Title, id)
	if err != nil {
		log.Printf("Update error: %v", err)
		http.Error(w, "Failed to update song", http.StatusInternalServerError)
		return
	}

	// Проверяем количество изменённых строк
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		log.Printf("❌ Песня ID %d не найдена, обновление не выполнено", id)
		http.Error(w, "Песня не найдена", http.StatusNotFound)
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
	idStr, ok := vars["id"]
	if !ok {
		http.Error(w, "ID не указан", http.StatusBadRequest)
		return
	}

	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid song ID", http.StatusBadRequest)
		return
	}

	// Проверяем, существует ли песня
	var exists bool
	err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM songs WHERE id=$1)", id).Scan(&exists)
	if err != nil {
		log.Printf("Ошибка проверки существования песни: %v", err)
		http.Error(w, "Ошибка сервера", http.StatusInternalServerError)
		return
	}

	if !exists {
		log.Printf("❌ Попытка удалить несуществующую песню ID %d", id)
		http.Error(w, "Песня не найдена", http.StatusNotFound)
		return
	}

	// Удаление песни
	log.Printf("🔍 Попытка удалить песню ID %d", id)
	query := `DELETE FROM songs WHERE id = $1`
	result, err := db.Exec(query, id)
	if err != nil {
		log.Printf("❌ Ошибка при удалении песни ID %d: %v", id, err)
		http.Error(w, "Ошибка сервера", http.StatusInternalServerError)
		return
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		log.Printf("❌ Ошибка: Песня ID %d не найдена при удалении", id)
		http.Error(w, "Песня не найдена", http.StatusNotFound)
		return
	}
	log.Printf("✅ Песня ID %d успешно удалена", id)
	w.WriteHeader(http.StatusNoContent)
}

// GetSongVersesHandler возвращает куплеты песни с пагинацией
// @Summary Получить куплеты песни
// @Description Возвращает текст песни, разбитый на куплеты (по \n\n) с пагинацией
// @Tags Songs
// @Produce json
// @Param id path int true "ID песни"
// @Param page query int false "Номер страницы (начиная с 1)" default(1)
// @Param perPage query int false "Количество куплетов на странице" default(1)
// @Success 200 {object} models.VerseResponse
// @Failure 400 {string} string "Invalid request"
// @Failure 404 {string} string "Song not found"
// @Router /songs/{id}/verses [get]
func getSongVersesHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		log.Printf("❌ Некорректный ID песни: %s", idStr)
		http.Error(w, "Invalid song ID", http.StatusBadRequest)
		return
	}

	// Чтение параметров page и perPage
	page := 1
	perPage := 1

	if p := r.URL.Query().Get("page"); p != "" {
		if val, err := strconv.Atoi(p); err == nil && val > 0 {
			page = val
		}
	}
	if pp := r.URL.Query().Get("perPage"); pp != "" {
		if val, err := strconv.Atoi(pp); err == nil && val > 0 {
			perPage = val
		}
	}

	log.Printf("📖 Запрос куплетов для песни ID %d (page: %d, perPage: %d)", id, page, perPage)

	// Запрашиваем текст песни
	var text sql.NullString
	err = db.QueryRow("SELECT text FROM songs WHERE id = $1", id).Scan(&text)
	if err == sql.ErrNoRows {
		log.Printf("❌ Песня ID %d не найдена", id)
		http.Error(w, "Песня не найдена", http.StatusNotFound)
		return
	} else if err != nil {
		log.Printf("❌ Ошибка при получении текста песни ID %d: %v", id, err)
		http.Error(w, "Ошибка сервера", http.StatusInternalServerError)
		return
	}

	if !text.Valid || text.String == "" {
		log.Printf("⚠️ Песня ID %d не содержит текста", id)
		http.Error(w, "У песни нет текста", http.StatusNotFound)
		return
	}

	// Разбиваем по \n\n (куплеты)
	verses := strings.Split(text.String, "\n\n")
	total := len(verses)

	// Пагинация
	start := (page - 1) * perPage
	end := start + perPage
	if start >= total {
		log.Printf("⚠️ Страница %d выходит за пределы куплетов (всего: %d)", page, total)
		http.Error(w, "Нет куплетов на этой странице", http.StatusNotFound)
		return
	}
	if end > total {
		end = total
	}

	response := models.VerseResponse{
		Page:        page,
		PerPage:     perPage,
		TotalVerses: total,
		Verses:      verses[start:end],
	}

	log.Printf("✅ Куплеты для песни ID %d — страница %d, отображено: %d куплета(ов) из %d", id, page, len(response.Verses), total)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Хэндлер главной страницы
func homeHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Добро пожаловать в Song Library API!")
}

// Запрос информации о песне из внешнего API
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
		return models.SongDetail{}, fmt.Errorf("Ошибка запроса к API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return models.SongDetail{}, fmt.Errorf("Внешний API вернул статус: %d", resp.StatusCode)
	}

	var detail models.SongDetail
	if err := json.NewDecoder(resp.Body).Decode(&detail); err != nil {
		return models.SongDetail{}, fmt.Errorf("Ошибка парсинга JSON: %w", err)
	}
	return detail, nil
}

// Миграция БД
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
		log.Fatalf("Ошибка миграции: %v", err)
	}
	log.Println("✅ Миграция завершена.")
}

func main() {
	// Загружаем конфигурацию
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Ошибка загрузки конфигурации: %v", err)
	}

	// Подключаемся к БД
	db, err = sql.Open("pgx", cfg.GetDatabaseURL())
	if err != nil {
		log.Fatalf("Ошибка подключения к БД: %v", err)
	}
	defer db.Close()

	// Проверяем соединение
	if err := db.Ping(); err != nil {
		log.Fatalf("Ошибка соединения с БД: %v", err)
	}
	log.Printf("✅ Подключено к PostgreSQL!")

	// Запускаем миграции
	migrate()

	// Настраиваем маршрутизатор
	r := mux.NewRouter()
	r.HandleFunc("/", homeHandler).Methods("GET")
	r.HandleFunc("/songs", addSongHandler).Methods("POST")
	r.HandleFunc("/songs", getSongsHandler).Methods("GET")
	r.HandleFunc("/songs/{id}", getSongByIDHandler).Methods("GET")
	r.HandleFunc("/songs/{id}", updateSongHandler).Methods("PUT")
	r.HandleFunc("/songs/{id}", deleteSongHandler).Methods("DELETE")
	r.PathPrefix("/swagger/").Handler(httpSwagger.WrapHandler)
	r.HandleFunc("/songs/{id}/verses", getSongVersesHandler).Methods("GET")

	// Запуск сервера
	port := "8080"
	log.Println("🚀 Сервер запущен на порту %s...")
	log.Fatal(http.ListenAndServe(":"+port, r))
}
