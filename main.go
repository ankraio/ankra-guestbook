// Ankra Guestbook — a deliberately small app that proves a real deployment:
// it renders, it talks to PostgreSQL, and it shows you which pod answered.
package main

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

//go:embed templates/index.html
var templateFS embed.FS

type entry struct {
	Name    string
	Message string
	Created time.Time
}

type pageData struct {
	Entries  []entry
	Pod      string
	DBStatus string
	DBError  string
	Count    int
	Version  string
}

var (
	db       *sql.DB
	tmpl     *template.Template
	version  = envOr("APP_VERSION", "1.0.0")
	podName  = envOr("POD_NAME", "local")
	listenOn = ":" + envOr("PORT", "8080")
)

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// databaseURL assembles a DSN from either DATABASE_URL or the discrete
// PG* variables a Helm chart or operator secret usually provides.
func databaseURL() string {
	if url := os.Getenv("DATABASE_URL"); url != "" {
		return url
	}
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		envOr("PGHOST", "localhost"),
		envOr("PGPORT", "5432"),
		envOr("PGUSER", "postgres"),
		os.Getenv("PGPASSWORD"),
		envOr("PGDATABASE", "guestbook"),
		envOr("PGSSLMODE", "disable"),
	)
}

const schema = `
CREATE TABLE IF NOT EXISTS entries (
  id      SERIAL PRIMARY KEY,
  name    TEXT NOT NULL,
  message TEXT NOT NULL,
  created TIMESTAMPTZ NOT NULL DEFAULT now()
);`

// connect retries because the app almost always starts before the database
// finishes accepting connections. Crash-looping on that is noise, not signal.
func connect(ctx context.Context) error {
	var lastErr error
	for attempt := 1; attempt <= 30; attempt++ {
		handle, err := sql.Open("postgres", databaseURL())
		if err != nil {
			lastErr = err
		} else {
			pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
			err = handle.PingContext(pingCtx)
			cancel()
			if err == nil {
				handle.SetMaxOpenConns(5)
				handle.SetConnMaxLifetime(5 * time.Minute)
				db = handle
				_, err = db.ExecContext(ctx, schema)
				return err
			}
			lastErr = err
			_ = handle.Close()
		}
		log.Printf("database not ready (attempt %d/30): %v", attempt, lastErr)
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("database unreachable after 30 attempts: %w", lastErr)
}

func loadEntries(ctx context.Context) ([]entry, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT name, message, created FROM entries ORDER BY created DESC LIMIT 50`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []entry
	for rows.Next() {
		var item entry
		if err := rows.Scan(&item.Name, &item.Message, &item.Created); err != nil {
			return nil, err
		}
		entries = append(entries, item)
	}
	return entries, rows.Err()
}

func handleIndex(writer http.ResponseWriter, request *http.Request) {
	if request.URL.Path != "/" {
		http.NotFound(writer, request)
		return
	}

	data := pageData{Pod: podName, Version: version, DBStatus: "connected"}

	if request.Method == http.MethodPost {
		name := strings.TrimSpace(request.FormValue("name"))
		message := strings.TrimSpace(request.FormValue("message"))
		if name == "" {
			name = "anonymous"
		}
		if len(name) > 60 {
			name = name[:60]
		}
		if len(message) > 500 {
			message = message[:500]
		}
		if message != "" {
			if _, err := db.ExecContext(request.Context(),
				`INSERT INTO entries (name, message) VALUES ($1, $2)`, name, message); err != nil {
				log.Printf("insert failed: %v", err)
			}
		}
		http.Redirect(writer, request, "/", http.StatusSeeOther)
		return
	}

	entries, err := loadEntries(request.Context())
	if err != nil {
		data.DBStatus = "error"
		data.DBError = err.Error()
	}
	data.Entries = entries
	data.Count = len(entries)

	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(writer, data); err != nil {
		log.Printf("render failed: %v", err)
	}
}

// handleHealthz is the readiness gate: not ready until the database answers.
func handleHealthz(writer http.ResponseWriter, request *http.Request) {
	if db == nil {
		http.Error(writer, "database not connected", http.StatusServiceUnavailable)
		return
	}
	ctx, cancel := context.WithTimeout(request.Context(), 2*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		http.Error(writer, "database unreachable", http.StatusServiceUnavailable)
		return
	}
	fmt.Fprintln(writer, "ok")
}

func main() {
	var err error
	tmpl, err = template.ParseFS(templateFS, "templates/index.html")
	if err != nil {
		log.Fatalf("parse template: %v", err)
	}

	ctx := context.Background()
	if err := connect(ctx); err != nil {
		log.Fatalf("startup: %v", err)
	}
	log.Printf("connected to postgres, serving on %s as pod %s", listenOn, podName)

	mux := http.NewServeMux()
	mux.HandleFunc("/", handleIndex)
	mux.HandleFunc("/healthz", handleHealthz)

	server := &http.Server{
		Addr:              listenOn,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("serve: %v", err)
	}
}
