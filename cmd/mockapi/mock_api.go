package main

import (
	"encoding/json"
	"log"
	"net/http"
	"songLibraryApi/internal/models"
)

func infoHandler(w http.ResponseWriter, r *http.Request) {
	group := r.URL.Query().Get("group")
	song := r.URL.Query().Get("song")

	log.Printf("🎧 Получен запрос от клиента: group=%s, song=%s", group, song)

	// фейковые данные
	if group == "Muse" && song == "Supermassive Black Hole" {
		response := models.SongDetail{
			ReleaseDate: "16.07.2006",
			Text:        "Ooh baby, don't you know I suffer?\nOoh baby, can you hear me moan?",
			Link:        "https://youtube.com/watch?v=Xsp3_a-PMTw",
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	} else {
		log.Printf("❌ Не найдено: group=%s, song=%s", group, song)
		http.Error(w, "Song not found", http.StatusNotFound)
	}
}

func main() {

	http.HandleFunc("/info", infoHandler)

	log.Println("🎤 Фейковый API запущен на http://mockapi:8081")
	log.Fatal(http.ListenAndServe(":8081", nil))
}
