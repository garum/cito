package main

import (
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	_ "github.com/lib/pq"
	"golang.org/x/oauth2"
)

func main() {

	// Build connection string from environment variables
	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_NAME"),
	)

	// Open connection
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		slog.Error("Failed to open database connection", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	// Verify connection
	err = db.Ping()
	if err != nil {
		slog.Error("Failed to ping database", "error", err)
		os.Exit(1)
	}
	fmt.Println("Successfully connected!")

	// Create users table
	createTableSQL := `
		CREATE TABLE IF NOT EXISTS users (
			id SERIAL PRIMARY KEY,
			github_id BIGINT UNIQUE NOT NULL,
			username VARCHAR(255) NOT NULL,
			email VARCHAR(255),
			access_token VARCHAR(255),
			session_token VARCHAR(255) UNIQUE
		);
	`
	_, err = db.Exec(createTableSQL)
	if err != nil {
		slog.Error("Failed to create users table", "error", err)
		os.Exit(1)
	}
	fmt.Println("Users table ready!")

	conf := &oauth2.Config{
		ClientID:     os.Getenv("GITHUB_CLIENT_ID"),
		ClientSecret: os.Getenv("GITHUB_CLIENT_SECRET"),
		Scopes:       []string{"user:email"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://github.com/login/oauth/authorize",
			TokenURL: "https://github.com/login/oauth/access_token",
		},
		RedirectURL: os.Getenv("GITHUB_REDIRECT_URL"),
	}

	app := NewApp(conf, db)

	mux := http.NewServeMux()

	app.RegisterRoutes(mux)

	server := http.Server{Addr: os.Getenv("SERVER_ADDR"), Handler: mux}
	slog.Info("Server listening", "addr", server.Addr)
	if err := server.ListenAndServe(); err != nil {
		slog.Error("Server failed", "error", err)
		os.Exit(1)
	}
	slog.Info("Server closed")
}
