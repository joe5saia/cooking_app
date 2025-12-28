package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestBootstrapToken(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/auth/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		var payload map[string]string
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode login payload: %v", err)
		}
		if payload["username"] != "sam" || payload["password"] != "pw" {
			t.Fatalf("unexpected login payload")
		}
		http.SetCookie(w, &http.Cookie{Name: "cooking_app_session", Value: "sess", Path: "/"})
		http.SetCookie(w, &http.Cookie{Name: "cooking_app_session_csrf", Value: "csrf123", Path: "/"})
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/api/v1/tokens", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get(csrfHeaderName); got != "csrf123" {
			t.Fatalf("csrf header = %q, want %q", got, "csrf123")
		}
		var payload CreateTokenRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode token payload: %v", err)
		}
		if payload.Name != "cookctl" {
			t.Fatalf("token name = %q, want %q", payload.Name, "cookctl")
		}
		resp := CreateTokenResponse{
			ID:        "token-1",
			Name:      payload.Name,
			Token:     "pat_abc",
			CreatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		}
		w.Header().Set("Content-Type", "application/json")
		writeJSON(t, w, resp)
	})
	mux.HandleFunc("/api/v1/auth/logout", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get(csrfHeaderName); got != "csrf123" {
			t.Fatalf("csrf header = %q, want %q", got, "csrf123")
		}
		w.WriteHeader(http.StatusNoContent)
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	sessionClient, err := NewSessionClient(server.URL, 5*time.Second, false, nil)
	if err != nil {
		t.Fatalf("NewSessionClient returned error: %v", err)
	}

	resp, err := sessionClient.BootstrapToken(context.Background(), "sam", "pw", "cookctl", nil)
	if err != nil {
		t.Fatalf("BootstrapToken returned error: %v", err)
	}
	if resp.Token != "pat_abc" {
		t.Fatalf("Token = %q, want %q", resp.Token, "pat_abc")
	}
}

func TestCSRFTokenMissing(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/auth/login", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "cooking_app_session", Value: "sess", Path: "/"})
		w.WriteHeader(http.StatusNoContent)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	sessionClient, err := NewSessionClient(server.URL, 5*time.Second, false, nil)
	if err != nil {
		t.Fatalf("NewSessionClient returned error: %v", err)
	}

	err = sessionClient.login(context.Background(), "sam", "pw")
	if err != nil {
		t.Fatalf("login returned error: %v", err)
	}
	if _, err := sessionClient.csrfToken(); err == nil || !strings.Contains(err.Error(), "csrf token") {
		t.Fatalf("expected csrf token error, got %v", err)
	}
}
