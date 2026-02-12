package testutil

import (
	"database/sql"
	"os"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

// SetupMockDB creates a mock database for testing
func SetupMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock, func()) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}

	cleanup := func() {
		db.Close()
	}

	return db, mock, cleanup
}

// NewMockRows is a helper to create sqlmock.Rows (re-export for convenience)
var NewMockRows = sqlmock.NewRows

// SetTestEnv sets environment variables for testing and returns a cleanup function
func SetTestEnv(t *testing.T) func() {
	// Save original values
	originalEnv := map[string]string{
		"DB_HOST":              os.Getenv("DB_HOST"),
		"DB_PORT":              os.Getenv("DB_PORT"),
		"DB_USER":              os.Getenv("DB_USER"),
		"DB_PASSWORD":          os.Getenv("DB_PASSWORD"),
		"DB_NAME":              os.Getenv("DB_NAME"),
		"GITHUB_CLIENT_ID":     os.Getenv("GITHUB_CLIENT_ID"),
		"GITHUB_CLIENT_SECRET": os.Getenv("GITHUB_CLIENT_SECRET"),
		"GITHUB_REDIRECT_URL":  os.Getenv("GITHUB_REDIRECT_URL"),
		"SERVER_ADDR":          os.Getenv("SERVER_ADDR"),
	}

	// Set test values
	os.Setenv("DB_HOST", "localhost")
	os.Setenv("DB_PORT", "5433")
	os.Setenv("DB_USER", "test")
	os.Setenv("DB_PASSWORD", "test")
	os.Setenv("DB_NAME", "cito_test")
	os.Setenv("GITHUB_CLIENT_ID", "test_client_id")
	os.Setenv("GITHUB_CLIENT_SECRET", "test_secret")
	os.Setenv("GITHUB_REDIRECT_URL", "http://localhost:8080/oauth2/callback")
	os.Setenv("SERVER_ADDR", "127.0.0.1:0")

	// Return cleanup function
	return func() {
		for k, v := range originalEnv {
			if v == "" {
				os.Unsetenv(k)
			} else {
				os.Setenv(k, v)
			}
		}
	}
}