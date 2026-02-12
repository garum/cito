package service

import (
	"cito/server/model"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"log/slog"
)

type UserService struct {
	db *sql.DB
}

func NewUserService(db *sql.DB) *UserService {
	return &UserService{db: db}
}

// generateSessionToken creates a random hex session token
func generateSessionToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// UpsertUser inserts or updates a user based on GitHub ID
func (us *UserService) UpsertUser(githubUser model.GitHubUser, accessToken string) (string, error) {
	sessionToken, err := generateSessionToken()
	if err != nil {
		return "", err
	}

	query := `
		INSERT INTO users (github_id, username, email, access_token, session_token)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (github_id)
		DO UPDATE SET
			username = EXCLUDED.username,
			email = EXCLUDED.email,
			access_token = EXCLUDED.access_token,
			session_token = EXCLUDED.session_token
		RETURNING session_token
	`

	var returnedToken string
	err = us.db.QueryRow(query, githubUser.ID, githubUser.Login, githubUser.Email, accessToken, sessionToken).Scan(&returnedToken)
	if err != nil {
		return "", err
	}

	slog.Info("Upserted user", "github_id", githubUser.ID, "username", githubUser.Login, "session_token", returnedToken)
	return returnedToken, nil
}

// FindUserBySession looks up a user by their session token
func (us *UserService) FindUserBySession(sessionToken string) (*model.UserModel, error) {
	query := `SELECT id, github_id, username, email, access_token, session_token FROM users WHERE session_token = $1`

	var user model.UserModel
	err := us.db.QueryRow(query, sessionToken).Scan(
		&user.ID,
		&user.GithubID,
		&user.Username,
		&user.Email,
		&user.AccessToken,
		&user.SessionToken,
	)

	if err != nil {
		return nil, err
	}

	return &user, nil
}
