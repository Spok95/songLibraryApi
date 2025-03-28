package models

import "database/sql"

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

func ToResponse(s Song) SongResponse {
	return SongResponse{
		ID:    s.ID,
		Group: s.Group,
		Title: s.Title,
		Text:  NullableToString(s.Text),
		Link:  NullableToString(s.Link),
		Date:  NullableToString(s.Date),
	}
}

func NullableToString(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}
