package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

type Book struct {
	ID        int64     `json:"id"`
	Title     string    `json:"title"`
	Author    string    `json:"author"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type bookPayload struct {
	Title  string `json:"title"`
	Author string `json:"author"`
}

type server struct {
	db *sql.DB
}

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL is required, e.g. postgres://user:pass@localhost:5432/dbname?sslmode=disable")
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("ping db: %v", err)
	}

	if err := ensureSchema(db); err != nil {
		log.Fatalf("ensure schema: %v", err)
	}

	srv := &server{db: db}

	http.HandleFunc("/books", srv.handleBooks)
	http.HandleFunc("/books/", srv.handleBookByID)

	addr := ":" + getEnv("PORT", "8080")
	log.Printf("listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

func ensureSchema(db *sql.DB) error {
	const ddl = `
CREATE TABLE IF NOT EXISTS books (
	id SERIAL PRIMARY KEY,
	title TEXT NOT NULL,
	author TEXT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
`
	if _, err := db.Exec(ddl); err != nil {
		return fmt.Errorf("create table: %w", err)
	}
	return nil
}

func (s *server) handleBooks(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.listBooks(w, r)
	case http.MethodPost:
		s.createBook(w, r)
	default:
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *server) handleBookByID(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/books/")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid id")
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.getBook(w, r, id)
	case http.MethodPut:
		s.updateBook(w, r, id)
	case http.MethodDelete:
		s.deleteBook(w, r, id)
	default:
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *server) listBooks(w http.ResponseWriter, _ *http.Request) {
	rows, err := s.db.Query(`SELECT id, title, author, created_at, updated_at FROM books ORDER BY id`)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	defer rows.Close()

	var books []Book
	for rows.Next() {
		var b Book
		if err := rows.Scan(&b.ID, &b.Title, &b.Author, &b.CreatedAt, &b.UpdatedAt); err != nil {
			respondInternalError(w, err)
			return
		}
		books = append(books, b)
	}

	respondJSON(w, http.StatusOK, books)
}

func (s *server) getBook(w http.ResponseWriter, _ *http.Request, id int64) {
	var b Book
	err := s.db.QueryRow(`SELECT id, title, author, created_at, updated_at FROM books WHERE id = $1`, id).
		Scan(&b.ID, &b.Title, &b.Author, &b.CreatedAt, &b.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		respondError(w, http.StatusNotFound, "book not found")
		return
	}
	if err != nil {
		respondInternalError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, b)
}

func (s *server) createBook(w http.ResponseWriter, r *http.Request) {
	var payload bookPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respondError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if payload.Title == "" || payload.Author == "" {
		respondError(w, http.StatusBadRequest, "title and author are required")
		return
	}

	var b Book
	err := s.db.QueryRow(
		`INSERT INTO books (title, author) VALUES ($1, $2) RETURNING id, title, author, created_at, updated_at`,
		payload.Title, payload.Author,
	).Scan(&b.ID, &b.Title, &b.Author, &b.CreatedAt, &b.UpdatedAt)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	respondJSON(w, http.StatusCreated, b)
}

func (s *server) updateBook(w http.ResponseWriter, r *http.Request, id int64) {
	var payload bookPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respondError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if payload.Title == "" || payload.Author == "" {
		respondError(w, http.StatusBadRequest, "title and author are required")
		return
	}

	var b Book
	err := s.db.QueryRow(
		`UPDATE books SET title = $1, author = $2, updated_at = NOW() WHERE id = $3 RETURNING id, title, author, created_at, updated_at`,
		payload.Title, payload.Author, id,
	).Scan(&b.ID, &b.Title, &b.Author, &b.CreatedAt, &b.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		respondError(w, http.StatusNotFound, "book not found")
		return
	}
	if err != nil {
		respondInternalError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, b)
}

func (s *server) deleteBook(w http.ResponseWriter, _ *http.Request, id int64) {
	res, err := s.db.Exec(`DELETE FROM books WHERE id = $1`, id)
	if err != nil {
		respondInternalError(w, err)
		return
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		respondError(w, http.StatusNotFound, "book not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func respondJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("write response: %v", err)
	}
}

func respondError(w http.ResponseWriter, status int, msg string) {
	respondJSON(w, status, map[string]string{"error": msg})
}

func respondInternalError(w http.ResponseWriter, err error) {
	log.Printf("internal error: %v", err)
	respondError(w, http.StatusInternalServerError, "internal server error")
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func StatusInternalServerError(w http.ResponseWriter){
	w.WriteHeader(http.StatusInternalServerError)
	w.Write([]byte("Internal Server Error"))
}