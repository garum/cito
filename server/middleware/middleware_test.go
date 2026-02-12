package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoggingMiddleware(t *testing.T) {
	tests := []struct {
		name          string
		method        string
		path          string
		handlerStatus int
		handlerBody   string
		wantStatus    int
		wantBody      string
		handlerCalled bool
	}{
		{
			name:          "middleware wraps GET request",
			method:        "GET",
			path:          "/test",
			handlerStatus: http.StatusOK,
			handlerBody:   "OK",
			wantStatus:    http.StatusOK,
			wantBody:      "OK",
			handlerCalled: true,
		},
		{
			name:          "middleware wraps POST request",
			method:        "POST",
			path:          "/api/data",
			handlerStatus: http.StatusCreated,
			handlerBody:   "Created",
			wantStatus:    http.StatusCreated,
			wantBody:      "Created",
			handlerCalled: true,
		},
		{
			name:          "middleware preserves handler errors",
			method:        "GET",
			path:          "/error",
			handlerStatus: http.StatusInternalServerError,
			handlerBody:   "Internal Server Error",
			wantStatus:    http.StatusInternalServerError,
			wantBody:      "Internal Server Error",
			handlerCalled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handlerCalled := false

			// Create a mock handler that the middleware will wrap
			mockHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handlerCalled = true
				w.WriteHeader(tt.handlerStatus)
				w.Write([]byte(tt.handlerBody))
			})

			// Wrap the mock handler with logging middleware
			wrappedHandler := LoggingMiddleware(mockHandler)

			// Create test request
			req := httptest.NewRequest(tt.method, tt.path, nil)
			rec := httptest.NewRecorder()

			// Execute the wrapped handler
			wrappedHandler.ServeHTTP(rec, req)

			// Verify the handler was called
			assert.Equal(t, tt.handlerCalled, handlerCalled, "handler should be called")

			// Verify the response
			assert.Equal(t, tt.wantStatus, rec.Code, "status code should match")
			assert.Equal(t, tt.wantBody, rec.Body.String(), "response body should match")
		})
	}
}

func TestLoggingMiddleware_ChainMultipleHandlers(t *testing.T) {
	// Create a chain of handlers to verify middleware doesn't break the chain
	finalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("final handler"))
	})

	// Wrap with logging middleware
	wrappedHandler := LoggingMiddleware(finalHandler)

	req := httptest.NewRequest("GET", "/chain", nil)
	rec := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "final handler", rec.Body.String())
}
