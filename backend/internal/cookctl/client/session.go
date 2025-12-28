// Package client provides session-based helpers for bootstrapping PATs.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"
)

const (
	csrfHeaderName   = "X-CSRF-Token"
	csrfCookieSuffix = "_csrf"
)

// SessionClient manages session-cookie requests for bootstrapping PATs.
type SessionClient struct {
	baseURL     *url.URL
	httpClient  *http.Client
	debug       bool
	debugWriter io.Writer
}

// CreateTokenRequest defines the PAT creation payload.
type CreateTokenRequest struct {
	Name      string     `json:"name"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// NewSessionClient constructs a session client with cookie jar support.
func NewSessionClient(baseURL string, timeout time.Duration, debug bool, debugWriter io.Writer) (*SessionClient, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid api url: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("invalid api url: %s", baseURL)
	}

	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("create cookie jar: %w", err)
	}

	if debugWriter == nil {
		debugWriter = io.Discard
	}

	return &SessionClient{
		baseURL: parsed,
		httpClient: &http.Client{
			Timeout: timeout,
			Jar:     jar,
		},
		debug:       debug,
		debugWriter: debugWriter,
	}, nil
}

// BootstrapToken logs in with a session cookie, creates a PAT, and logs out.
func (c *SessionClient) BootstrapToken(ctx context.Context, username, password, name string, expiresAt *time.Time) (CreateTokenResponse, error) {
	if err := c.login(ctx, username, password); err != nil {
		return CreateTokenResponse{}, err
	}

	csrfToken, err := c.csrfToken()
	if err != nil {
		return CreateTokenResponse{}, err
	}

	resp, err := c.createToken(ctx, csrfToken, CreateTokenRequest{
		Name:      name,
		ExpiresAt: expiresAt,
	})
	if err != nil {
		return CreateTokenResponse{}, err
	}

	if err := c.logout(ctx, csrfToken); err != nil && c.debug {
		c.debugf("logout failed: %v\n", err)
	}

	return resp, nil
}

func (c *SessionClient) login(ctx context.Context, username, password string) error {
	reqBody := map[string]string{
		"username": username,
		"password": password,
	}
	return c.doJSON(ctx, http.MethodPost, "/api/v1/auth/login", "", reqBody, nil)
}

func (c *SessionClient) createToken(ctx context.Context, csrfToken string, req CreateTokenRequest) (CreateTokenResponse, error) {
	var out CreateTokenResponse
	if err := c.doJSON(ctx, http.MethodPost, "/api/v1/tokens", csrfToken, req, &out); err != nil {
		return CreateTokenResponse{}, err
	}
	return out, nil
}

func (c *SessionClient) logout(ctx context.Context, csrfToken string) error {
	return c.doJSON(ctx, http.MethodPost, "/api/v1/auth/logout", csrfToken, nil, nil)
}

func (c *SessionClient) doJSON(ctx context.Context, method, path, csrfToken string, body interface{}, out interface{}) error {
	var payload io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode request: %w", err)
		}
		payload = bytes.NewReader(raw)
	}

	req, err := c.newRequest(ctx, method, path, payload)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if csrfToken != "" {
		req.Header.Set(csrfHeaderName, csrfToken)
	}

	resp, err := c.do(req)
	if err != nil {
		return err
	}
	defer c.closeBody(resp)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return readAPIError(resp)
	}
	if out == nil || resp.StatusCode == http.StatusNoContent {
		return nil
	}

	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

func (c *SessionClient) newRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	rel := &url.URL{Path: path}
	u := c.baseURL.ResolveReference(rel)
	req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	return req, nil
}

func (c *SessionClient) do(req *http.Request) (*http.Response, error) {
	if c.debug {
		c.debugf("request %s %s\n", req.Method, req.URL.String())
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if c.debug {
		c.debugf("response %s %s -> %d\n", req.Method, req.URL.String(), resp.StatusCode)
	}
	return resp, nil
}

func (c *SessionClient) debugf(format string, args ...any) {
	if c.debugWriter == nil {
		return
	}
	if _, err := fmt.Fprintf(c.debugWriter, format, args...); err != nil {
		// Best-effort debug logging only.
		_ = err
	}
}

func (c *SessionClient) closeBody(resp *http.Response) {
	if resp == nil || resp.Body == nil {
		return
	}
	if err := resp.Body.Close(); err != nil && c.debug {
		c.debugf("close body error: %v\n", err)
	}
}

func (c *SessionClient) csrfToken() (string, error) {
	if c.httpClient.Jar == nil {
		return "", errors.New("cookie jar is required for csrf token lookup")
	}
	cookies := c.httpClient.Jar.Cookies(c.baseURL)
	for _, cookie := range cookies {
		if strings.HasSuffix(cookie.Name, csrfCookieSuffix) && cookie.Value != "" {
			return cookie.Value, nil
		}
	}
	return "", errors.New("csrf token not found")
}
