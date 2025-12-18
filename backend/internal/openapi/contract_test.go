package openapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers/legacy"
	"github.com/saiaj/cooking_app/backend/internal/bootstrap"
	"github.com/saiaj/cooking_app/backend/internal/config"
	"github.com/saiaj/cooking_app/backend/internal/db/sqlc"
	"github.com/saiaj/cooking_app/backend/internal/httpapi"
	"github.com/saiaj/cooking_app/backend/internal/httpapi/response"
	"github.com/saiaj/cooking_app/backend/internal/logging"
	"github.com/saiaj/cooking_app/backend/internal/testutil/pgtest"
)

const (
	testSessionCookieName = "cooking_app_session"
	testCSRFCookieName    = testSessionCookieName + "_csrf"
)

func loadOpenAPI(t *testing.T) *openapi3.T {
	t.Helper()

	path := filepath.Join("..", "..", "openapi.yaml")
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromFile(path)
	if err != nil {
		t.Fatalf("load openapi: %v", err)
	}
	if err := doc.Validate(loader.Context); err != nil {
		t.Fatalf("validate openapi: %v", err)
	}
	return doc
}

func startServer(t *testing.T) (*httptest.Server, *http.Client) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)

	postgres := pgtest.Start(ctx, t)
	db := postgres.OpenSQL(ctx, t)
	postgres.MigrateUp(ctx, t, db)

	pool := postgres.NewPool(ctx, t)
	queries := sqlc.New(pool)
	if _, err := bootstrap.CreateFirstUser(ctx, queries, bootstrap.FirstUserParams{
		Username:    "joe",
		Password:    "pw",
		DisplayName: nil,
	}); err != nil {
		t.Fatalf("bootstrap user: %v", err)
	}

	app, err := httpapi.New(ctx, logging.New("error"), config.Config{
		DatabaseURL:                postgres.DatabaseURL,
		LogLevel:                   "error",
		SessionCookieName:          testSessionCookieName,
		SessionTTL:                 24 * time.Hour,
		SessionCookieSecure:        false,
		MaxJSONBodyBytes:           2 << 20,
		StrictJSON:                 true,
		LoginRateLimitPerMin:       0,
		LoginRateLimitBurst:        0,
		TokenCreateRateLimitPerMin: 0,
		TokenCreateRateLimitBurst:  0,
	})
	if err != nil {
		t.Fatalf("new app: %v", err)
	}
	t.Cleanup(app.Close)

	server := httptest.NewServer(app.Handler())
	t.Cleanup(server.Close)

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookie jar: %v", err)
	}
	client := &http.Client{Jar: jar}

	return server, client
}

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

	reqBody := `{"username":"joe","password":"pw"}`
	resp, err := client.Post(baseURL+"/api/v1/auth/login", "application/json", strings.NewReader(reqBody))
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
		t.Fatalf("client with cookie jar is required")
	}
	csrf := cookieValue(t, client.Jar, baseURL, testCSRFCookieName)
	if csrf == "" {
		t.Fatalf("missing csrf cookie")
	}
	return csrf
}

func validateOpenAPIRequestResponse(t *testing.T, doc *openapi3.T, req *http.Request, resp *http.Response, respBody []byte) {
	t.Helper()

	router, err := legacy.NewRouter(doc)
	if err != nil {
		t.Fatalf("new router: %v", err)
	}

	route, pathParams, err := router.FindRoute(req)
	if err != nil {
		t.Fatalf("find route: %v", err)
	}

	opts := &openapi3filter.Options{
		AuthenticationFunc: func(context.Context, *openapi3filter.AuthenticationInput) error {
			return nil
		},
	}

	ctx := context.Background()
	reqInput := &openapi3filter.RequestValidationInput{
		Request:    req,
		PathParams: pathParams,
		Route:      route,
		Options:    opts,
	}
	if err := openapi3filter.ValidateRequest(ctx, reqInput); err != nil {
		t.Fatalf("validate request: %v", err)
	}

	respInput := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: reqInput,
		Status:                 resp.StatusCode,
		Header:                 resp.Header,
		Body:                   io.NopCloser(bytes.NewReader(respBody)),
		Options:                opts,
	}
	if err := openapi3filter.ValidateResponse(ctx, respInput); err != nil {
		t.Fatalf("validate response: %v", err)
	}
}

func TestOpenAPI_OperationsHaveExpectedSecurityAndResponses(t *testing.T) {
	t.Parallel()

	doc := loadOpenAPI(t)

	type expected struct {
		method            string
		path              string
		requiresAuth      bool
		requiredResponses []string
	}

	tests := []expected{
		{method: http.MethodPost, path: "/api/v1/auth/login", requiresAuth: false, requiredResponses: []string{"204", "400", "401", "429", "500"}},
		{method: http.MethodPost, path: "/api/v1/auth/logout", requiresAuth: true, requiredResponses: []string{"204", "401", "500"}},
		{method: http.MethodGet, path: "/api/v1/auth/me", requiresAuth: true, requiredResponses: []string{"200", "401", "500"}},
		{method: http.MethodGet, path: "/api/v1/tokens", requiresAuth: true, requiredResponses: []string{"200", "401", "500"}},
		{method: http.MethodPost, path: "/api/v1/tokens", requiresAuth: true, requiredResponses: []string{"200", "400", "401", "429", "500"}},
		{method: http.MethodDelete, path: "/api/v1/tokens/{id}", requiresAuth: true, requiredResponses: []string{"204", "400", "401", "404", "500"}},
		{method: http.MethodGet, path: "/api/v1/users", requiresAuth: true, requiredResponses: []string{"200", "401", "500"}},
		{method: http.MethodPost, path: "/api/v1/users", requiresAuth: true, requiredResponses: []string{"200", "400", "401", "500"}},
		{method: http.MethodPut, path: "/api/v1/users/{id}/deactivate", requiresAuth: true, requiredResponses: []string{"204", "400", "401", "404", "500"}},
		{method: http.MethodGet, path: "/api/v1/tags", requiresAuth: true, requiredResponses: []string{"200", "401", "500"}},
		{method: http.MethodPost, path: "/api/v1/tags", requiresAuth: true, requiredResponses: []string{"200", "400", "401", "500"}},
		{method: http.MethodGet, path: "/api/v1/recipe-books", requiresAuth: true, requiredResponses: []string{"200", "401", "500"}},
		{method: http.MethodPost, path: "/api/v1/recipe-books", requiresAuth: true, requiredResponses: []string{"200", "400", "401", "500"}},
		{method: http.MethodGet, path: "/api/v1/recipes", requiresAuth: true, requiredResponses: []string{"200", "401", "500"}},
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			pathItem := doc.Paths.Find(tt.path)
			if pathItem == nil {
				t.Fatalf("missing path: %s", tt.path)
			}

			var op *openapi3.Operation
			switch tt.method {
			case http.MethodGet:
				op = pathItem.Get
			case http.MethodPost:
				op = pathItem.Post
			case http.MethodPut:
				op = pathItem.Put
			case http.MethodDelete:
				op = pathItem.Delete
			default:
				t.Fatalf("unhandled method: %s", tt.method)
			}
			if op == nil {
				t.Fatalf("missing operation: %s %s", tt.method, tt.path)
			}

			hasSecurity := op.Security != nil && len(*op.Security) > 0
			if tt.requiresAuth && !hasSecurity {
				t.Fatalf("expected security requirements")
			}
			if !tt.requiresAuth && hasSecurity {
				t.Fatalf("expected no security requirements")
			}

			for _, code := range tt.requiredResponses {
				if op.Responses == nil || op.Responses.Value(code) == nil {
					t.Fatalf("missing response %s", code)
				}
				respRef := op.Responses.Value(code)
				if respRef.Value == nil {
					t.Fatalf("missing response value for %s", code)
				}
				if code == "204" {
					continue
				}
				mt := respRef.Value.Content.Get("application/json")
				if mt == nil || mt.Schema == nil {
					t.Fatalf("missing application/json schema for response %s", code)
				}
			}
		})
	}
}

func TestOpenAPI_RequestResponseValidation_PerMajorTagGroup(t *testing.T) {
	doc := loadOpenAPI(t)
	server, client := startServer(t)

	t.Run("auth login success", func(t *testing.T) {
		reqBody := `{"username":"joe","password":"pw"}`
		req, err := http.NewRequest(http.MethodPost, server.URL+"/api/v1/auth/login", strings.NewReader(reqBody))
		if err != nil {
			t.Fatalf("new request: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")

		validateReq, err := http.NewRequest(http.MethodPost, server.URL+"/api/v1/auth/login", strings.NewReader(reqBody))
		if err != nil {
			t.Fatalf("new validate request: %v", err)
		}
		validateReq.Header = req.Header.Clone()

		validateOpenAPIRequestResponseReadBody(t, doc, validateReq, mustDo(t, client, req))
	})

	t.Run("auth login unauthorized", func(t *testing.T) {
		reqBody := `{"username":"joe","password":"wrong"}`
		req, err := http.NewRequest(http.MethodPost, server.URL+"/api/v1/auth/login", strings.NewReader(reqBody))
		if err != nil {
			t.Fatalf("new request: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")

		validateReq, err := http.NewRequest(http.MethodPost, server.URL+"/api/v1/auth/login", strings.NewReader(reqBody))
		if err != nil {
			t.Fatalf("new validate request: %v", err)
		}
		validateReq.Header = req.Header.Clone()

		resp := mustDo(t, http.DefaultClient, req)
		respBody := mustReadAll(t, resp.Body)
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Errorf("close body: %v", closeErr)
		}
		validateOpenAPIRequestResponse(t, doc, validateReq, resp, respBody)
	})

	csrf := loginAndGetCSRFToken(t, client, server.URL)

	t.Run("tokens create", func(t *testing.T) {
		reqBody := `{"name":"cli"}`
		req := mustNewJSONRequest(t, server.URL+"/api/v1/tokens", reqBody)
		req.Header.Set("X-CSRF-Token", csrf)

		validateReq := mustNewJSONRequest(t, server.URL+"/api/v1/tokens", reqBody)
		validateReq.Header = req.Header.Clone()

		resp := mustDo(t, client, req)
		respBody := mustReadAll(t, resp.Body)
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Errorf("close body: %v", closeErr)
		}
		validateOpenAPIRequestResponse(t, doc, validateReq, resp, respBody)
	})

	t.Run("users list", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, server.URL+"/api/v1/users", nil)
		if err != nil {
			t.Fatalf("new request: %v", err)
		}
		validateReq, err := http.NewRequest(http.MethodGet, server.URL+"/api/v1/users", nil)
		if err != nil {
			t.Fatalf("new validate request: %v", err)
		}
		resp := mustDo(t, client, req)
		respBody := mustReadAll(t, resp.Body)
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Errorf("close body: %v", closeErr)
		}
		validateOpenAPIRequestResponse(t, doc, validateReq, resp, respBody)
	})

	t.Run("tags create", func(t *testing.T) {
		reqBody := `{"name":"Soup"}`
		req := mustNewJSONRequest(t, server.URL+"/api/v1/tags", reqBody)
		req.Header.Set("X-CSRF-Token", csrf)

		validateReq := mustNewJSONRequest(t, server.URL+"/api/v1/tags", reqBody)
		validateReq.Header = req.Header.Clone()

		resp := mustDo(t, client, req)
		respBody := mustReadAll(t, resp.Body)
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Errorf("close body: %v", closeErr)
		}
		validateOpenAPIRequestResponse(t, doc, validateReq, resp, respBody)
	})

	t.Run("recipe books create", func(t *testing.T) {
		reqBody := `{"name":"Dinner"}`
		req := mustNewJSONRequest(t, server.URL+"/api/v1/recipe-books", reqBody)
		req.Header.Set("X-CSRF-Token", csrf)

		validateReq := mustNewJSONRequest(t, server.URL+"/api/v1/recipe-books", reqBody)
		validateReq.Header = req.Header.Clone()

		resp := mustDo(t, client, req)
		respBody := mustReadAll(t, resp.Body)
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Errorf("close body: %v", closeErr)
		}
		validateOpenAPIRequestResponse(t, doc, validateReq, resp, respBody)
	})

	t.Run("recipes list", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, server.URL+"/api/v1/recipes", nil)
		if err != nil {
			t.Fatalf("new request: %v", err)
		}
		validateReq, err := http.NewRequest(http.MethodGet, server.URL+"/api/v1/recipes", nil)
		if err != nil {
			t.Fatalf("new validate request: %v", err)
		}
		resp := mustDo(t, client, req)
		respBody := mustReadAll(t, resp.Body)
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Errorf("close body: %v", closeErr)
		}
		validateOpenAPIRequestResponse(t, doc, validateReq, resp, respBody)
	})
}

func mustDo(t *testing.T, client *http.Client, req *http.Request) *http.Response {
	t.Helper()
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	return resp
}

func mustReadAll(t *testing.T, r io.Reader) []byte {
	t.Helper()
	b, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return b
}

func mustNewJSONRequest(t *testing.T, urlStr, body string) *http.Request {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, urlStr, strings.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	return req
}

func validateOpenAPIRequestResponseReadBody(t *testing.T, doc *openapi3.T, req *http.Request, resp *http.Response) {
	t.Helper()
	respBody := mustReadAll(t, resp.Body)
	if closeErr := resp.Body.Close(); closeErr != nil {
		t.Errorf("close body: %v", closeErr)
	}
	validateOpenAPIRequestResponse(t, doc, req, resp, respBody)
}

func TestOpenAPI_ProblemSchemaMatchesResponseType(t *testing.T) {
	t.Parallel()

	doc := loadOpenAPI(t)
	s := doc.Components.Schemas["Problem"]
	if s == nil || s.Value == nil {
		t.Fatalf("missing Problem schema")
	}

	b, err := json.Marshal(response.Problem{Code: "unauthorized", Message: "unauthorized"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if err := s.Value.VisitJSON(v); err != nil {
		t.Fatalf("Problem schema does not accept response.Problem: %v", err)
	}
}
