package httpapi

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
	"time"
)

func (a *App) newCSRFToken() (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw[:]), nil
}

func (a *App) setCSRFCookie(w http.ResponseWriter, token string, expiresAt time.Time) error {
	if w == nil {
		return errors.New("response writer is required")
	}
	if strings.TrimSpace(token) == "" {
		return errors.New("csrf token is required")
	}

	http.SetCookie(w, &http.Cookie{
		Name:     a.csrfCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: false,
		Secure:   a.sessionCookieSecure,
		SameSite: http.SameSiteLaxMode,
		Expires:  expiresAt,
	})
	return nil
}

func (a *App) clearCSRFCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     a.csrfCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: false,
		Secure:   a.sessionCookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	})
}

func (a *App) csrfTokenFromRequest(r *http.Request) (cookieToken, headerToken string) {
	if r == nil {
		return "", ""
	}

	c, err := r.Cookie(a.csrfCookieName)
	if err == nil {
		cookieToken = c.Value
	}
	headerToken = strings.TrimSpace(r.Header.Get(a.csrfHeaderName))

	return cookieToken, headerToken
}

func (a *App) isCSRFValid(r *http.Request) bool {
	cookieToken, headerToken := a.csrfTokenFromRequest(r)
	if cookieToken == "" || headerToken == "" {
		return false
	}
	if len(cookieToken) != len(headerToken) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(cookieToken), []byte(headerToken)) == 1
}

func isUnsafeMethod(method string) bool {
	switch strings.ToUpper(strings.TrimSpace(method)) {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}
