package main

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	"cito/server/model"
	"cito/server/service"
	"cito/server/testutil"
)

func TestApp_loginHandler(t *testing.T) {
	tests := []struct {
		name             string
		authCodeURL      string
		wantStatus       int
		wantBodyContains string
	}{
		{
			name:             "returns HTML with OAuth URL",
			authCodeURL:      "https://github.com/login/oauth/authorize?client_id=test&state=state",
			wantStatus:       http.StatusOK,
			wantBodyContains: `<a href="https://github.com/login/oauth/authorize?client_id=test&state=state">Sign in with GitHub</a>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockOAuth := &testutil.MockOAuth2Config{
				AuthCodeURLFunc: func(state string, opts ...oauth2.AuthCodeOption) string {
					return tt.authCodeURL
				},
			}

			app := &App{
				oauthConfig: mockOAuth,
			}

			req := httptest.NewRequest("GET", "/login", nil)
			rec := httptest.NewRecorder()

			app.loginHandler(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code, "status code should match")
			assert.Contains(t, rec.Body.String(), tt.wantBodyContains, "body should contain OAuth URL")
		})
	}
}

func TestApp_homeHandler(t *testing.T) {
	tests := []struct {
		name             string
		sessionCookie    *http.Cookie
		mockUserService  func() *service.UserService
		wantStatus       int
		wantLocation     string
		wantBodyContains string
	}{
		{
			name:          "no session cookie redirects to login",
			sessionCookie: nil,
			mockUserService: func() *service.UserService {
				return &service.UserService{}
			},
			wantStatus:   http.StatusMovedPermanently,
			wantLocation: "/login",
		},
		{
			name: "invalid session redirects to login",
			sessionCookie: &http.Cookie{
				Name:  "session_token",
				Value: "invalid_token",
			},
			mockUserService: func() *service.UserService {
				db, mock, _ := testutil.SetupMockDB(t)
				mock.ExpectQuery(`SELECT (.+) FROM users WHERE session_token`).
					WithArgs("invalid_token").
					WillReturnError(sql.ErrNoRows)
				return service.NewUserService(db)
			},
			wantStatus:   http.StatusMovedPermanently,
			wantLocation: "/login",
		},
		{
			name: "valid session shows greeting",
			sessionCookie: &http.Cookie{
				Name:  "session_token",
				Value: "valid_token",
			},
			mockUserService: func() *service.UserService {
				db, mock, _ := testutil.SetupMockDB(t)
				rows := testutil.NewMockRows([]string{"id", "github_id", "username", "email", "access_token", "session_token"}).
					AddRow(1, int64(12345), "testuser", "test@example.com", "token", "valid_token")
				mock.ExpectQuery(`SELECT (.+) FROM users WHERE session_token`).
					WithArgs("valid_token").
					WillReturnRows(rows)
				return service.NewUserService(db)
			},
			wantStatus:       http.StatusOK,
			wantBodyContains: "Hello, testuser!",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &App{
				userService: tt.mockUserService(),
			}

			req := httptest.NewRequest("GET", "/", nil)
			if tt.sessionCookie != nil {
				req.AddCookie(tt.sessionCookie)
			}

			rec := httptest.NewRecorder()
			app.homeHandler(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code, "status code should match")

			if tt.wantLocation != "" {
				assert.Equal(t, tt.wantLocation, rec.Header().Get("Location"), "redirect location should match")
			}

			if tt.wantBodyContains != "" {
				assert.Contains(t, rec.Body.String(), tt.wantBodyContains, "body should contain expected text")
			}
		})
	}
}

func TestApp_callBackHandler(t *testing.T) {
	tests := []struct {
		name            string
		code            string
		mockOAuth       *testutil.MockOAuth2Config
		mockHTTPClient  *testutil.MockHTTPClient
		mockUserService func() *service.UserService
		wantStatus      int
		wantLocation    string
		wantCookie      bool
	}{
		{
			name: "successful OAuth flow",
			code: "valid_code",
			mockOAuth: &testutil.MockOAuth2Config{
				ExchangeFunc: func(ctx context.Context, code string, opts ...oauth2.AuthCodeOption) (*oauth2.Token, error) {
					return &oauth2.Token{AccessToken: "github_access_token"}, nil
				},
			},
			mockHTTPClient: testutil.NewMockHTTPClient(http.StatusOK, `{"id":12345,"login":"testuser","email":"test@example.com"}`),
			mockUserService: func() *service.UserService {
				db, mock, _ := testutil.SetupMockDB(t)
				mock.ExpectQuery(`INSERT INTO users`).
					WillReturnRows(testutil.NewMockRows([]string{"session_token"}).AddRow("new_session_token"))
				return service.NewUserService(db)
			},
			wantStatus:   http.StatusTemporaryRedirect,
			wantLocation: "/",
			wantCookie:   true,
		},
		{
			name: "OAuth exchange fails",
			code: "invalid_code",
			mockOAuth: &testutil.MockOAuth2Config{
				ExchangeFunc: func(ctx context.Context, code string, opts ...oauth2.AuthCodeOption) (*oauth2.Token, error) {
					return nil, assert.AnError
				},
			},
			mockHTTPClient: testutil.NewMockHTTPClient(http.StatusOK, ""),
			mockUserService: func() *service.UserService {
				return &service.UserService{}
			},
			wantStatus: http.StatusInternalServerError,
			wantCookie: false,
		},
		{
			name: "GitHub API returns error",
			code: "valid_code",
			mockOAuth: &testutil.MockOAuth2Config{
				ExchangeFunc: func(ctx context.Context, code string, opts ...oauth2.AuthCodeOption) (*oauth2.Token, error) {
					return &oauth2.Token{AccessToken: "github_access_token"}, nil
				},
			},
			mockHTTPClient: testutil.NewMockHTTPClient(http.StatusUnauthorized, "Unauthorized"),
			mockUserService: func() *service.UserService {
				return &service.UserService{}
			},
			wantStatus: http.StatusInternalServerError,
			wantCookie: false,
		},
		{
			name: "database upsert fails",
			code: "valid_code",
			mockOAuth: &testutil.MockOAuth2Config{
				ExchangeFunc: func(ctx context.Context, code string, opts ...oauth2.AuthCodeOption) (*oauth2.Token, error) {
					return &oauth2.Token{AccessToken: "github_access_token"}, nil
				},
			},
			mockHTTPClient: testutil.NewMockHTTPClient(http.StatusOK, `{"id":12345,"login":"testuser","email":"test@example.com"}`),
			mockUserService: func() *service.UserService {
				db, mock, _ := testutil.SetupMockDB(t)
				mock.ExpectQuery(`INSERT INTO users`).
					WillReturnError(sql.ErrConnDone)
				return service.NewUserService(db)
			},
			wantStatus: http.StatusInternalServerError,
			wantCookie: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &App{
				oauthConfig:  tt.mockOAuth,
				httpClient:   tt.mockHTTPClient,
				userService:  tt.mockUserService(),
				githubAPIURL: "https://api.github.com/user",
			}

			req := httptest.NewRequest("GET", "/oauth2/callback?code="+tt.code, nil)
			rec := httptest.NewRecorder()

			app.callBackHandler(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code, "status code should match")

			if tt.wantLocation != "" {
				assert.Equal(t, tt.wantLocation, rec.Header().Get("Location"), "redirect location should match")
			}

			if tt.wantCookie {
				cookies := rec.Result().Cookies()
				require.NotEmpty(t, cookies, "should have cookies set")
				found := false
				for _, cookie := range cookies {
					if cookie.Name == "session_token" {
						found = true
						assert.NotEmpty(t, cookie.Value, "session token should not be empty")
						assert.Equal(t, "/", cookie.Path, "cookie path should be /")
						assert.True(t, cookie.HttpOnly, "cookie should be HttpOnly")
						assert.Equal(t, 86400*7, cookie.MaxAge, "cookie max age should be 7 days")
					}
				}
				assert.True(t, found, "session_token cookie should be set")
			} else {
				cookies := rec.Result().Cookies()
				for _, cookie := range cookies {
					assert.NotEqual(t, "session_token", cookie.Name, "session_token cookie should not be set on error")
				}
			}
		})
	}
}

func TestApp_fetchGitHubUser(t *testing.T) {
	tests := []struct {
		name           string
		accessToken    string
		mockHTTPClient *testutil.MockHTTPClient
		githubAPIURL   string
		wantUser       *model.GitHubUser
		wantErr        bool
		errContains    string
	}{
		{
			name:           "successfully fetches GitHub user",
			accessToken:    "valid_token",
			mockHTTPClient: testutil.NewMockHTTPClient(http.StatusOK, `{"id":12345,"login":"testuser","email":"test@example.com"}`),
			githubAPIURL:   "https://api.github.com/user",
			wantUser: &model.GitHubUser{
				ID:    12345,
				Login: "testuser",
				Email: "test@example.com",
			},
			wantErr: false,
		},
		{
			name:           "handles non-200 status code",
			accessToken:    "invalid_token",
			mockHTTPClient: testutil.NewMockHTTPClient(http.StatusUnauthorized, "Unauthorized"),
			githubAPIURL:   "https://api.github.com/user",
			wantUser:       nil,
			wantErr:        true,
			errContains:    "status 401",
		},
		{
			name:           "handles invalid JSON",
			accessToken:    "valid_token",
			mockHTTPClient: testutil.NewMockHTTPClient(http.StatusOK, `invalid json`),
			githubAPIURL:   "https://api.github.com/user",
			wantUser:       nil,
			wantErr:        true,
			errContains:    "failed to parse",
		},
		{
			name:           "uses default URL when not set",
			accessToken:    "valid_token",
			mockHTTPClient: testutil.NewMockHTTPClient(http.StatusOK, `{"id":99999,"login":"defaultuser","email":"default@example.com"}`),
			githubAPIURL:   "", // Empty - should use default
			wantUser: &model.GitHubUser{
				ID:    99999,
				Login: "defaultuser",
				Email: "default@example.com",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &App{
				httpClient:   tt.mockHTTPClient,
				githubAPIURL: tt.githubAPIURL,
			}

			user, err := app.fetchGitHubUser(tt.accessToken)

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
				assert.Equal(t, tt.wantUser.Login, user.Login)
				assert.Equal(t, tt.wantUser.Email, user.Email)
			}
		})
	}
}
