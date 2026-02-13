//go:build integration
// +build integration

package tests

import (
	"cito/server/model"
	"cito/server/service"
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// setupTestDB creates a PostgreSQL test container and returns a connection
func setupTestDB(t *testing.T) (*sql.DB, func()) {
	ctx := context.Background()

	// Create PostgreSQL container
	pgContainer, err := postgres.RunContainer(ctx,
		testcontainers.WithImage("postgres:15-alpine"),
		postgres.WithDatabase("cito_test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second)),
	)
	require.NoError(t, err, "failed to start postgres container")

	// Get connection string
	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err, "failed to get connection string")

	// Open database connection
	db, err := sql.Open("postgres", connStr)
	require.NoError(t, err, "failed to open database connection")

	// Verify connection
	err = db.Ping()
	require.NoError(t, err, "failed to ping database")

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
	require.NoError(t, err, "failed to create users table")

	// Cleanup function
	cleanup := func() {
		db.Close()
		if err := pgContainer.Terminate(ctx); err != nil {
			t.Logf("failed to terminate container: %v", err)
		}
	}

	return db, cleanup
}

func TestIntegration_UserService_UpsertUser(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	us := service.NewUserService(db)

	githubUser := model.GitHubUser{
		ID:    12345,
		Login: "testuser",
		Email: "test@example.com",
	}

	t.Run("inserts new user on first call", func(t *testing.T) {
		token1, err := us.UpsertUser(githubUser, "access_token_1")
		require.NoError(t, err, "should insert user without error")
		assert.NotEmpty(t, token1, "should return session token")

		// Verify user exists in database
		var count int
		err = db.QueryRow("SELECT COUNT(*) FROM users WHERE github_id = $1", githubUser.ID).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count, "should have exactly one user")
	})

	t.Run("updates existing user on second call", func(t *testing.T) {
		// First insert
		token1, err := us.UpsertUser(githubUser, "access_token_1")
		require.NoError(t, err)

		// Second upsert with same GitHub ID but different data
		updatedUser := model.GitHubUser{
			ID:    12345, // Same GitHub ID
			Login: "updateduser",
			Email: "updated@example.com",
		}
		token2, err := us.UpsertUser(updatedUser, "access_token_2")
		require.NoError(t, err)
		assert.NotEmpty(t, token2)
		assert.NotEqual(t, token1, token2, "session token should be regenerated")

		// Verify only one user exists
		var count int
		err = db.QueryRow("SELECT COUNT(*) FROM users WHERE github_id = $1", githubUser.ID).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count, "should still have exactly one user")

		// Verify user data was updated
		var username, email, accessToken string
		err = db.QueryRow("SELECT username, email, access_token FROM users WHERE github_id = $1", githubUser.ID).
			Scan(&username, &email, &accessToken)
		require.NoError(t, err)
		assert.Equal(t, "updateduser", username)
		assert.Equal(t, "updated@example.com", email)
		assert.Equal(t, "access_token_2", accessToken)
	})
}

func TestIntegration_UserService_FindUserBySession(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	us := service.NewUserService(db)

	// Insert a test user
	githubUser := model.GitHubUser{
		ID:    99999,
		Login: "sessiontestuser",
		Email: "session@example.com",
	}
	sessionToken, err := us.UpsertUser(githubUser, "access_token")
	require.NoError(t, err)

	t.Run("finds user by valid session token", func(t *testing.T) {
		user, err := us.FindUserBySession(sessionToken)
		require.NoError(t, err, "should find user without error")
		require.NotNil(t, user)
		assert.Equal(t, int64(99999), user.GithubID)
		assert.Equal(t, "sessiontestuser", user.Username)
		assert.Equal(t, "session@example.com", user.Email)
		assert.Equal(t, sessionToken, user.SessionToken)
	})

	t.Run("returns error for invalid session token", func(t *testing.T) {
		user, err := us.FindUserBySession("invalid_token_12345")
		require.Error(t, err, "should return error for invalid token")
		assert.Nil(t, user)
		assert.Equal(t, sql.ErrNoRows, err)
	})

	t.Run("returns error for empty session token", func(t *testing.T) {
		user, err := us.FindUserBySession("")
		require.Error(t, err)
		assert.Nil(t, user)
	})
}

func TestIntegration_SessionTokenUniqueness(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	us := service.NewUserService(db)

	// Create multiple users
	users := []model.GitHubUser{
		{ID: 1001, Login: "user1", Email: "user1@example.com"},
		{ID: 1002, Login: "user2", Email: "user2@example.com"},
		{ID: 1003, Login: "user3", Email: "user3@example.com"},
	}

	tokens := make(map[string]bool)

	for _, user := range users {
		token, err := us.UpsertUser(user, "access_token")
		require.NoError(t, err)
		assert.NotEmpty(t, token)

		// Check token is unique
		assert.False(t, tokens[token], "session token should be unique across users")
		tokens[token] = true
	}

	// Verify all tokens are different
	assert.Equal(t, 3, len(tokens), "should have 3 unique session tokens")
}

func TestIntegration_DatabaseConstraints(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	us := service.NewUserService(db)

	t.Run("github_id unique constraint enforced", func(t *testing.T) {
		githubUser := model.GitHubUser{
			ID:    5555,
			Login: "constrainttest",
			Email: "constraint@example.com",
		}

		// First insert should succeed
		token1, err := us.UpsertUser(githubUser, "token1")
		require.NoError(t, err)
		assert.NotEmpty(t, token1)

		// Second upsert with same github_id should update, not create duplicate
		_, err = us.UpsertUser(githubUser, "token2")
		require.NoError(t, err)

		// Verify only one row exists
		var count int
		err = db.QueryRow("SELECT COUNT(*) FROM users WHERE github_id = $1", githubUser.ID).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count, "unique constraint should prevent duplicate github_id")
	})

	t.Run("session_token unique constraint enforced", func(t *testing.T) {
		// This is implicitly tested by the upsert mechanism
		// Session tokens are regenerated on each upsert, ensuring uniqueness
		user1 := model.GitHubUser{ID: 6001, Login: "user1", Email: "user1@test.com"}
		user2 := model.GitHubUser{ID: 6002, Login: "user2", Email: "user2@test.com"}

		token1, err := us.UpsertUser(user1, "token")
		require.NoError(t, err)

		token2, err2 := us.UpsertUser(user2, "token")
		require.NoError(t, err2)

		assert.NotEqual(t, token1, token2, "session tokens should be unique")
	})
}

func TestIntegration_ConcurrentOperations(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	us := service.NewUserService(db)

	// Test concurrent upserts of different users
	t.Run("concurrent upserts of different users", func(t *testing.T) {
		done := make(chan bool)

		for i := 0; i < 5; i++ {
			go func(id int) {
				user := model.GitHubUser{
					ID:    int64(7000 + id),
					Login: "concurrent" + string(rune('a'+id)),
					Email: "concurrent@test.com",
				}
				_, err := us.UpsertUser(user, "token")
				assert.NoError(t, err)
				done <- true
			}(i)
		}

		// Wait for all goroutines
		for i := 0; i < 5; i++ {
			<-done
		}

		// Verify all 5 users were created
		var count int
		err := db.QueryRow("SELECT COUNT(*) FROM users WHERE github_id >= 7000 AND github_id < 7010").Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 5, count, "all concurrent inserts should succeed")
	})
}

func TestIntegration_FullUserLifecycle(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	us := service.NewUserService(db)

	// Complete user lifecycle: create, update, find
	githubUser := model.GitHubUser{
		ID:    8888,
		Login: "lifecycleuser",
		Email: "lifecycle@example.com",
	}

	// Step 1: Create user
	token1, err := us.UpsertUser(githubUser, "initial_token")
	require.NoError(t, err, "should create user")
	assert.NotEmpty(t, token1)

	// Step 2: Find user by session token
	foundUser, err := us.FindUserBySession(token1)
	require.NoError(t, err, "should find user by session")
	assert.Equal(t, githubUser.ID, foundUser.GithubID)
	assert.Equal(t, githubUser.Login, foundUser.Username)
	assert.Equal(t, "initial_token", foundUser.AccessToken)

	// Step 3: Update user (new login session)
	updatedGithubUser := model.GitHubUser{
		ID:    8888, // Same ID
		Login: "lifecycleuser_updated",
		Email: "lifecycle_updated@example.com",
	}
	token2, err := us.UpsertUser(updatedGithubUser, "updated_token")
	require.NoError(t, err, "should update user")
	assert.NotEmpty(t, token2)
	assert.NotEqual(t, token1, token2, "new session should have new token")

	// Step 4: Old token should no longer work
	oldUser, err := us.FindUserBySession(token1)
	require.Error(t, err, "old session token should be invalid")
	assert.Nil(t, oldUser)

	// Step 5: New token should work
	newUser, err := us.FindUserBySession(token2)
	require.NoError(t, err, "new session token should work")
	require.NotNil(t, newUser)
	assert.Equal(t, "lifecycleuser_updated", newUser.Username)
	assert.Equal(t, "lifecycle_updated@example.com", newUser.Email)
	assert.Equal(t, "updated_token", newUser.AccessToken)
}
