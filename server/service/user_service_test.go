package service

import (
	"cito/server/model"
	"database/sql"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cito/server/testutil"
)

func TestGenerateSessionToken(t *testing.T) {
	// Test that session token is generated
	token, err := generateSessionToken()
	require.NoError(t, err, "should generate token without error")
	assert.NotEmpty(t, token, "token should not be empty")
	assert.Equal(t, 64, len(token), "token should be 64 characters (32 bytes hex encoded)")

	// Test that tokens are unique
	token2, err := generateSessionToken()
	require.NoError(t, err)
	assert.NotEqual(t, token, token2, "consecutive tokens should be unique")
}

func TestUserService_UpsertUser(t *testing.T) {
	tests := []struct {
		name        string
		githubUser  model.GitHubUser
		accessToken string
		mockSetup   func(sqlmock.Sqlmock)
		wantErr     bool
		errContains string
	}{
		{
			name: "successfully inserts new user",
			githubUser: model.GitHubUser{
				ID:    12345,
				Login: "testuser",
				Email: "test@example.com",
			},
			accessToken: "github_token_123",
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`INSERT INTO users`).
					WithArgs(int64(12345), "testuser", "test@example.com", "github_token_123", sqlmock.AnyArg()).
					WillReturnRows(sqlmock.NewRows([]string{"session_token"}).AddRow("generated_session_token"))
			},
			wantErr: false,
		},
		{
			name: "successfully updates existing user",
			githubUser: model.GitHubUser{
				ID:    12345,
				Login: "updateduser",
				Email: "updated@example.com",
			},
			accessToken: "new_github_token",
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`INSERT INTO users`).
					WithArgs(int64(12345), "updateduser", "updated@example.com", "new_github_token", sqlmock.AnyArg()).
					WillReturnRows(sqlmock.NewRows([]string{"session_token"}).AddRow("new_session_token"))
			},
			wantErr: false,
		},
		{
			name: "handles database error",
			githubUser: model.GitHubUser{
				ID:    12345,
				Login: "testuser",
				Email: "test@example.com",
			},
			accessToken: "github_token_123",
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`INSERT INTO users`).
					WithArgs(int64(12345), "testuser", "test@example.com", "github_token_123", sqlmock.AnyArg()).
					WillReturnError(sql.ErrConnDone)
			},
			wantErr:     true,
			errContains: "connection",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, cleanup := testutil.SetupMockDB(t)
			defer cleanup()

			tt.mockSetup(mock)

			us := NewUserService(db)
			token, err := us.UpsertUser(tt.githubUser, tt.accessToken)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				assert.Empty(t, token)
			} else {
				require.NoError(t, err)
				assert.NotEmpty(t, token, "should return session token")
			}

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestUserService_FindUserBySession(t *testing.T) {
	tests := []struct {
		name         string
		sessionToken string
		mockSetup    func(sqlmock.Sqlmock)
		wantUser     *model.UserModel
		wantErr      bool
		errContains  string
	}{
		{
			name:         "valid session token returns user",
			sessionToken: "valid_token_123",
			mockSetup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"id", "github_id", "username", "email", "access_token", "session_token"}).
					AddRow(1, int64(12345), "testuser", "test@example.com", "gh_token", "valid_token_123")
				mock.ExpectQuery(`SELECT (.+) FROM users WHERE session_token`).
					WithArgs("valid_token_123").
					WillReturnRows(rows)
			},
			wantUser: &model.UserModel{
				ID:           1,
				GithubID:     12345,
				Username:     "testuser",
				Email:        "test@example.com",
				AccessToken:  "gh_token",
				SessionToken: "valid_token_123",
			},
			wantErr: false,
		},
		{
			name:         "invalid session token returns error",
			sessionToken: "invalid_token",
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT (.+) FROM users WHERE session_token`).
					WithArgs("invalid_token").
					WillReturnError(sql.ErrNoRows)
			},
			wantUser:    nil,
			wantErr:     true,
			errContains: "no rows",
		},
		{
			name:         "database error returns error",
			sessionToken: "any_token",
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT (.+) FROM users WHERE session_token`).
					WithArgs("any_token").
					WillReturnError(sql.ErrConnDone)
			},
			wantUser:    nil,
			wantErr:     true,
			errContains: "connection",
		},
		{
			name:         "empty session token",
			sessionToken: "",
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT (.+) FROM users WHERE session_token`).
					WithArgs("").
					WillReturnError(sql.ErrNoRows)
			},
			wantUser:    nil,
			wantErr:     true,
			errContains: "no rows",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, cleanup := testutil.SetupMockDB(t)
			defer cleanup()

			tt.mockSetup(mock)

			us := NewUserService(db)
			user, err := us.FindUserBySession(tt.sessionToken)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				assert.Nil(t, user)
			} else {
				require.NoError(t, err)
				require.NotNil(t, user)
				assert.Equal(t, tt.wantUser.ID, user.ID)
				assert.Equal(t, tt.wantUser.GithubID, user.GithubID)
				assert.Equal(t, tt.wantUser.Username, user.Username)
				assert.Equal(t, tt.wantUser.Email, user.Email)
				assert.Equal(t, tt.wantUser.AccessToken, user.AccessToken)
				assert.Equal(t, tt.wantUser.SessionToken, user.SessionToken)
			}

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
