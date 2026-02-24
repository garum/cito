package handler

import (
	"cito/server/model"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHomeHandler_WithUser(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	user := &model.UserModel{Username: "testuser"}
	ctx := model.NewContextWithUserValue(req.Context(), user)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	HomeHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	body := rr.Body.String()
	expected := `<h1>Hello, testuser!</h1><p>You are logged in.</p>`
	if body != expected {
		t.Errorf("expected body %q, got %q", expected, body)
	}
}

func TestHomeHandler_WithoutUser(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	HomeHandler(rr, req)

	if rr.Code != http.StatusFound {
		t.Errorf("expected status %d, got %d", http.StatusFound, rr.Code)
	}

	location := rr.Header().Get("Location")
	if location != "/login" {
		t.Errorf("expected redirect to /login, got %q", location)
	}
}