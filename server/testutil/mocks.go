package testutil

import (
	"bytes"
	"context"
	"database/sql"
	"io"
	"net/http"

	"golang.org/x/oauth2"
)

// MockHTTPClient is a mock implementation of HTTPClient
type MockHTTPClient struct {
	DoFunc func(req *http.Request) (*http.Response, error)
}

func (m *MockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	if m.DoFunc != nil {
		return m.DoFunc(req)
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBufferString(`{}`)),
	}, nil
}

// NewMockHTTPClient creates a mock HTTP client with a custom response
func NewMockHTTPClient(statusCode int, body string) *MockHTTPClient {
	return &MockHTTPClient{
		DoFunc: func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: statusCode,
				Body:       io.NopCloser(bytes.NewBufferString(body)),
				Header:     make(http.Header),
			}, nil
		},
	}
}

// MockOAuth2Config is a mock implementation of OAuth2TokenExchanger
type MockOAuth2Config struct {
	ExchangeFunc    func(ctx context.Context, code string, opts ...oauth2.AuthCodeOption) (*oauth2.Token, error)
	AuthCodeURLFunc func(state string, opts ...oauth2.AuthCodeOption) string
}

func (m *MockOAuth2Config) Exchange(ctx context.Context, code string, opts ...oauth2.AuthCodeOption) (*oauth2.Token, error) {
	if m.ExchangeFunc != nil {
		return m.ExchangeFunc(ctx, code, opts...)
	}
	return &oauth2.Token{AccessToken: "mock_access_token"}, nil
}

func (m *MockOAuth2Config) AuthCodeURL(state string, opts ...oauth2.AuthCodeOption) string {
	if m.AuthCodeURLFunc != nil {
		return m.AuthCodeURLFunc(state, opts...)
	}
	return "https://github.com/login/oauth/authorize?client_id=test&state=" + state
}

// MockUserService is a mock implementation of UserService
type MockUserService struct {
	UpsertUserFunc        func(githubUser interface{}, accessToken string) (string, error)
	FindUserBySessionFunc func(sessionToken string) (interface{}, error)
}

func (m *MockUserService) UpsertUser(githubUser interface{}, accessToken string) (string, error) {
	if m.UpsertUserFunc != nil {
		return m.UpsertUserFunc(githubUser, accessToken)
	}
	return "mock_session_token", nil
}

func (m *MockUserService) FindUserBySession(sessionToken string) (interface{}, error) {
	if m.FindUserBySessionFunc != nil {
		return m.FindUserBySessionFunc(sessionToken)
	}
	return nil, sql.ErrNoRows
}
