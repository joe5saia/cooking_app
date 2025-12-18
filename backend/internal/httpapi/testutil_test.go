package httpapi_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

const (
	testSessionCookieName = "cooking_app_session"
	testCSRFCookieName    = testSessionCookieName + "_csrf"
)

func cookieValue(t *testing.T, jar http.CookieJar, baseURL, name string) string {
	t.Helper()

	u, err := url.Parse(baseURL)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}

	for _, c := range jar.Cookies(u) {
		if c.Name == name {
			return c.Value
		}
	}
	return ""
}

func loginAndGetCSRFToken(t *testing.T, client *http.Client, baseURL string) string {
	t.Helper()

	resp, err := client.Post(baseURL+"/api/v1/auth/login", "application/json", strings.NewReader(`{"username":"joe","password":"pw"}`))
	if err != nil {
		t.Fatalf("post login: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Errorf("close body: %v", closeErr)
		}
	})
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("login status=%d, want %d", resp.StatusCode, http.StatusNoContent)
	}

	if client == nil || client.Jar == nil {
		t.Fatalf("client with cookie jar is required to extract csrf token")
	}
	csrf := cookieValue(t, client.Jar, baseURL, testCSRFCookieName)
	if csrf == "" {
		t.Fatalf("missing csrf cookie")
	}

	return csrf
}

func newJSONRequest(t *testing.T, method, urlStr, body string) *http.Request {
	t.Helper()

	req, err := http.NewRequest(method, urlStr, strings.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	return req
}
