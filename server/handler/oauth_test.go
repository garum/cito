package handler

import (
	"bytes"
	"cito/server/service"
	"cito/server/testutil"
	"context"
	"database/sql"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

// roundTripFunc adapts a function to http.RoundTripper so we can mock *http.Client.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func mockHTTPClient(statusCode int, body string) *http.Client {
	return &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: statusCode,
				Body:       io.NopCloser(bytes.NewBufferString(body)),
				Header:     make(http.Header),
			}, nil
		}),
	}
}

func TestOAuthHandler_LoginHandler(t *testing.T) {
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

			authService := service.NewAuthService(mockOAuth, nil)
			userService := service.NewUserService(nil)
			handler := NewOAuthHandler(authService, userService)

			req := httptest.NewRequest("GET", "/login", nil)
			rec := httptest.NewRecorder()

			handler.LoginHandler(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code, "status code should match")
			assert.Contains(t, rec.Body.String(), tt.wantBodyContains, "body should contain OAuth URL")
		})
	}
}

func TestOAuthHandler_CallBackHandler(t *testing.T) {
	tests := []struct {
		name         string
		code         string
		mockOAuth    *testutil.MockOAuth2Config
		httpClient   *http.Client
		setupMockDB  func() (*sql.DB, func())
		wantStatus   int
		wantLocation string
		wantCookie   bool
	}{
		{
			name: "successful OAuth flow",
			code: "valid_code",
			mockOAuth: &testutil.MockOAuth2Config{
				ExchangeFunc: func(ctx context.Context, code string, opts ...oauth2.AuthCodeOption) (*oauth2.Token, error) {
					return &oauth2.Token{AccessToken: "github_access_token"}, nil
				},
			},
			httpClient: mockHTTPClient(http.StatusOK, `{"id":12345,"login":"testuser","email":"test@example.com"}`),
			setupMockDB: func() (*sql.DB, func()) {
				db, mock, cleanup := testutil.SetupMockDB(t)
				mock.ExpectQuery(`INSERT INTO users`).
					WillReturnRows(testutil.NewMockRows([]string{"session_token"}).AddRow("new_session_token"))
				return db, cleanup
			},
			wantStatus:   http.StatusFound,
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
			httpClient:  mockHTTPClient(http.StatusOK, ""),
			setupMockDB: nil,
			wantStatus:  http.StatusInternalServerError,
			wantCookie:  false,
		},
		{
			name: "GitHub API returns error",
			code: "valid_code",
			mockOAuth: &testutil.MockOAuth2Config{
				ExchangeFunc: func(ctx context.Context, code string, opts ...oauth2.AuthCodeOption) (*oauth2.Token, error) {
					return &oauth2.Token{AccessToken: "github_access_token"}, nil
				},
			},
			httpClient:  mockHTTPClient(http.StatusUnauthorized, "Unauthorized"),
			setupMockDB: nil,
			wantStatus:  http.StatusInternalServerError,
			wantCookie:  false,
		},
		{
			name: "database upsert fails",
			code: "valid_code",
			mockOAuth: &testutil.MockOAuth2Config{
				ExchangeFunc: func(ctx context.Context, code string, opts ...oauth2.AuthCodeOption) (*oauth2.Token, error) {
					return &oauth2.Token{AccessToken: "github_access_token"}, nil
				},
			},
			httpClient: mockHTTPClient(http.StatusOK, `{"id":12345,"login":"testuser","email":"test@example.com"}`),
			setupMockDB: func() (*sql.DB, func()) {
				db, mock, cleanup := testutil.SetupMockDB(t)
				mock.ExpectQuery(`INSERT INTO users`).
					WillReturnError(sql.ErrConnDone)
				return db, cleanup
			},
			wantStatus: http.StatusInternalServerError,
			wantCookie: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var db *sql.DB
			if tt.setupMockDB != nil {
				var cleanup func()
				db, cleanup = tt.setupMockDB()
				defer cleanup()
			}

			authService := service.NewAuthService(tt.mockOAuth, tt.httpClient)
			userService := service.NewUserService(db)
			handler := NewOAuthHandler(authService, userService)

			req := httptest.NewRequest("GET", "/oauth2/callback?code="+tt.code, nil)
			rec := httptest.NewRecorder()

			handler.CallBackHandler(rec, req)

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