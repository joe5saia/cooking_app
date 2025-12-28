// Package client provides typed access to the Cooking App API for cookctl.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client wraps HTTP operations against the Cooking App API.
type Client struct {
	baseURL     *url.URL
	token       string
	httpClient  *http.Client
	debug       bool
	debugWriter io.Writer
}

// HealthResponse represents the API health response.
type HealthResponse struct {
	OK bool `json:"ok"`
}

// MeResponse represents the authenticated user response.
type MeResponse struct {
	ID          string  `json:"id"`
	Username    string  `json:"username"`
	DisplayName *string `json:"display_name"`
}

// Token represents a personal access token summary.
type Token struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
	ExpiresAt  *time.Time `json:"expires_at"`
}

// CreateTokenResponse mirrors the PAT creation response.
type CreateTokenResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Token     string    `json:"token"`
	CreatedAt time.Time `json:"created_at"`
}

// Tag represents a tag in the cooking app.
type Tag struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

// RecipeBook represents a recipe book.
type RecipeBook struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

// User represents a user in the system.
type User struct {
	ID          string    `json:"id"`
	Username    string    `json:"username"`
	DisplayName *string   `json:"display_name"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
}

// RecipeTag represents a tag attached to a recipe.
type RecipeTag struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// RecipeListItem is a summary of a recipe for list responses.
type RecipeListItem struct {
	ID               string      `json:"id"`
	Title            string      `json:"title"`
	Servings         int         `json:"servings"`
	PrepTimeMinutes  int         `json:"prep_time_minutes"`
	TotalTimeMinutes int         `json:"total_time_minutes"`
	SourceURL        *string     `json:"source_url"`
	Notes            *string     `json:"notes"`
	RecipeBookID     *string     `json:"recipe_book_id"`
	Tags             []RecipeTag `json:"tags"`
	DeletedAt        *time.Time  `json:"deleted_at"`
	UpdatedAt        time.Time   `json:"updated_at"`
}

// RecipeIngredient represents an ingredient line on a recipe detail.
type RecipeIngredient struct {
	ID           string   `json:"id"`
	Position     int      `json:"position"`
	Quantity     *float64 `json:"quantity"`
	QuantityText *string  `json:"quantity_text"`
	Unit         *string  `json:"unit"`
	Item         string   `json:"item"`
	Prep         *string  `json:"prep"`
	Notes        *string  `json:"notes"`
	OriginalText *string  `json:"original_text"`
}

// RecipeStep represents a recipe instruction step.
type RecipeStep struct {
	ID          string `json:"id"`
	StepNumber  int    `json:"step_number"`
	Instruction string `json:"instruction"`
}

// RecipeDetail represents a full recipe detail response.
type RecipeDetail struct {
	ID               string             `json:"id"`
	Title            string             `json:"title"`
	Servings         int                `json:"servings"`
	PrepTimeMinutes  int                `json:"prep_time_minutes"`
	TotalTimeMinutes int                `json:"total_time_minutes"`
	SourceURL        *string            `json:"source_url"`
	Notes            *string            `json:"notes"`
	RecipeBookID     *string            `json:"recipe_book_id"`
	Tags             []RecipeTag        `json:"tags"`
	Ingredients      []RecipeIngredient `json:"ingredients"`
	Steps            []RecipeStep       `json:"steps"`
	CreatedAt        time.Time          `json:"created_at"`
	CreatedBy        string             `json:"created_by"`
	UpdatedAt        time.Time          `json:"updated_at"`
	UpdatedBy        string             `json:"updated_by"`
	DeletedAt        *time.Time         `json:"deleted_at"`
}

// RecipeListResponse represents the paginated recipe list response.
type RecipeListResponse struct {
	Items      []RecipeListItem `json:"items"`
	NextCursor *string          `json:"next_cursor"`
}

// MealPlanRecipe represents a recipe summary attached to a meal plan entry.
type MealPlanRecipe struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// MealPlanEntry represents a single meal plan entry for a date.
type MealPlanEntry struct {
	Date   string         `json:"date"`
	Recipe MealPlanRecipe `json:"recipe"`
}

// MealPlanListResponse represents the meal plan entries in a date range.
type MealPlanListResponse struct {
	Items []MealPlanEntry `json:"items"`
}

// RecipeListParams defines optional filters for listing recipes.
type RecipeListParams struct {
	Query          string
	BookID         string
	TagID          string
	IncludeDeleted bool
	Limit          int
	Cursor         string
}

// Problem matches the API error response format.
type Problem struct {
	Code    string       `json:"code"`
	Message string       `json:"message"`
	Details []FieldError `json:"details,omitempty"`
}

// FieldError contains field-level validation details.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// APIError represents a non-2xx API response.
type APIError struct {
	StatusCode int
	Problem    Problem
}

// Error implements the error interface.
func (e *APIError) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Problem.Code != "" {
		return fmt.Sprintf("api error: %s", e.Problem.Code)
	}
	return fmt.Sprintf("api error: status %d", e.StatusCode)
}

// UserMessage returns a CLI-friendly error message.
func (e *APIError) UserMessage() string {
	if e == nil {
		return ""
	}
	if e.Problem.Code == "" {
		return fmt.Sprintf("request failed with status %d", e.StatusCode)
	}
	if e.Problem.Code != "validation_error" || len(e.Problem.Details) == 0 {
		return fmt.Sprintf("%s: %s", e.Problem.Code, e.Problem.Message)
	}
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("%s: %s", e.Problem.Code, e.Problem.Message))
	for _, detail := range e.Problem.Details {
		builder.WriteString(fmt.Sprintf("\nfield=%s message=%s", detail.Field, detail.Message))
	}
	return builder.String()
}

// New constructs a client with the provided configuration.
func New(baseURL, token string, timeout time.Duration, debug bool, debugWriter io.Writer) (*Client, error) {
	if baseURL == "" {
		return nil, errors.New("api url is required")
	}
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid api url: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("invalid api url: %s", baseURL)
	}
	if debugWriter == nil {
		debugWriter = io.Discard
	}
	return &Client{
		baseURL: parsed,
		token:   token,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		debug:       debug,
		debugWriter: debugWriter,
	}, nil
}

// Health checks the API health endpoint.
func (c *Client) Health(ctx context.Context) (HealthResponse, error) {
	var out HealthResponse
	if err := c.doJSON(ctx, http.MethodGet, "/api/v1/healthz", nil, &out); err != nil {
		return HealthResponse{}, err
	}
	return out, nil
}

// Me returns the currently authenticated user.
func (c *Client) Me(ctx context.Context) (MeResponse, error) {
	var out MeResponse
	if err := c.doJSON(ctx, http.MethodGet, "/api/v1/auth/me", nil, &out); err != nil {
		return MeResponse{}, err
	}
	return out, nil
}

// Tokens lists personal access tokens.
func (c *Client) Tokens(ctx context.Context) ([]Token, error) {
	var out []Token
	if err := c.doJSON(ctx, http.MethodGet, "/api/v1/tokens", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// CreateToken creates a new personal access token.
func (c *Client) CreateToken(ctx context.Context, name string, expiresAt *time.Time) (CreateTokenResponse, error) {
	payload := struct {
		Name      string     `json:"name"`
		ExpiresAt *time.Time `json:"expires_at,omitempty"`
	}{
		Name:      name,
		ExpiresAt: expiresAt,
	}

	var out CreateTokenResponse
	if err := c.doJSON(ctx, http.MethodPost, "/api/v1/tokens", payload, &out); err != nil {
		return CreateTokenResponse{}, err
	}
	return out, nil
}

// RevokeToken revokes a personal access token by id.
func (c *Client) RevokeToken(ctx context.Context, id string) error {
	path := fmt.Sprintf("/api/v1/tokens/%s", id)
	return c.doJSON(ctx, http.MethodDelete, path, nil, nil)
}

// Tags lists tags.
func (c *Client) Tags(ctx context.Context) ([]Tag, error) {
	var out []Tag
	if err := c.doJSON(ctx, http.MethodGet, "/api/v1/tags", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// CreateTag creates a new tag.
func (c *Client) CreateTag(ctx context.Context, name string) (Tag, error) {
	payload := struct {
		Name string `json:"name"`
	}{
		Name: name,
	}
	var out Tag
	if err := c.doJSON(ctx, http.MethodPost, "/api/v1/tags", payload, &out); err != nil {
		return Tag{}, err
	}
	return out, nil
}

// UpdateTag updates an existing tag by id.
func (c *Client) UpdateTag(ctx context.Context, id, name string) (Tag, error) {
	payload := struct {
		Name string `json:"name"`
	}{
		Name: name,
	}
	path := fmt.Sprintf("/api/v1/tags/%s", id)
	var out Tag
	if err := c.doJSON(ctx, http.MethodPut, path, payload, &out); err != nil {
		return Tag{}, err
	}
	return out, nil
}

// DeleteTag deletes a tag by id.
func (c *Client) DeleteTag(ctx context.Context, id string) error {
	path := fmt.Sprintf("/api/v1/tags/%s", id)
	return c.doJSON(ctx, http.MethodDelete, path, nil, nil)
}

// RecipeBooks lists recipe books.
func (c *Client) RecipeBooks(ctx context.Context) ([]RecipeBook, error) {
	var out []RecipeBook
	if err := c.doJSON(ctx, http.MethodGet, "/api/v1/recipe-books", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// CreateRecipeBook creates a new recipe book.
func (c *Client) CreateRecipeBook(ctx context.Context, name string) (RecipeBook, error) {
	payload := struct {
		Name string `json:"name"`
	}{
		Name: name,
	}
	var out RecipeBook
	if err := c.doJSON(ctx, http.MethodPost, "/api/v1/recipe-books", payload, &out); err != nil {
		return RecipeBook{}, err
	}
	return out, nil
}

// UpdateRecipeBook updates a recipe book by id.
func (c *Client) UpdateRecipeBook(ctx context.Context, id, name string) (RecipeBook, error) {
	payload := struct {
		Name string `json:"name"`
	}{
		Name: name,
	}
	path := fmt.Sprintf("/api/v1/recipe-books/%s", id)
	var out RecipeBook
	if err := c.doJSON(ctx, http.MethodPut, path, payload, &out); err != nil {
		return RecipeBook{}, err
	}
	return out, nil
}

// DeleteRecipeBook deletes a recipe book by id.
func (c *Client) DeleteRecipeBook(ctx context.Context, id string) error {
	path := fmt.Sprintf("/api/v1/recipe-books/%s", id)
	return c.doJSON(ctx, http.MethodDelete, path, nil, nil)
}

// Users lists users.
func (c *Client) Users(ctx context.Context) ([]User, error) {
	var out []User
	if err := c.doJSON(ctx, http.MethodGet, "/api/v1/users", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// CreateUser creates a new user.
func (c *Client) CreateUser(ctx context.Context, username, password string, displayName *string) (User, error) {
	payload := struct {
		Username    string  `json:"username"`
		Password    string  `json:"password"`
		DisplayName *string `json:"display_name,omitempty"`
	}{
		Username:    username,
		Password:    password,
		DisplayName: displayName,
	}
	var out User
	if err := c.doJSON(ctx, http.MethodPost, "/api/v1/users", payload, &out); err != nil {
		return User{}, err
	}
	return out, nil
}

// DeactivateUser deactivates a user by id.
func (c *Client) DeactivateUser(ctx context.Context, id string) error {
	path := fmt.Sprintf("/api/v1/users/%s/deactivate", id)
	return c.doJSON(ctx, http.MethodPut, path, nil, nil)
}

// Recipes lists recipes with optional filters.
func (c *Client) Recipes(ctx context.Context, params RecipeListParams) (RecipeListResponse, error) {
	query := url.Values{}
	if params.Query != "" {
		query.Set("q", params.Query)
	}
	if params.BookID != "" {
		query.Set("book_id", params.BookID)
	}
	if params.TagID != "" {
		query.Set("tag_id", params.TagID)
	}
	if params.IncludeDeleted {
		query.Set("include_deleted", "true")
	}
	if params.Limit > 0 {
		query.Set("limit", fmt.Sprintf("%d", params.Limit))
	}
	if params.Cursor != "" {
		query.Set("cursor", params.Cursor)
	}

	var out RecipeListResponse
	if err := c.doJSONWithQuery(ctx, http.MethodGet, "/api/v1/recipes", query, nil, &out); err != nil {
		return RecipeListResponse{}, err
	}
	return out, nil
}

// MealPlans lists meal plan entries for a date range (inclusive).
func (c *Client) MealPlans(ctx context.Context, start, end string) (MealPlanListResponse, error) {
	query := url.Values{}
	query.Set("start", start)
	query.Set("end", end)

	var out MealPlanListResponse
	if err := c.doJSONWithQuery(ctx, http.MethodGet, "/api/v1/meal-plans", query, nil, &out); err != nil {
		return MealPlanListResponse{}, err
	}
	return out, nil
}

// CreateMealPlan adds a recipe to the meal plan on the given date.
func (c *Client) CreateMealPlan(ctx context.Context, date, recipeID string) (MealPlanEntry, error) {
	payload := struct {
		Date     string `json:"date"`
		RecipeID string `json:"recipe_id"`
	}{
		Date:     date,
		RecipeID: recipeID,
	}

	var out MealPlanEntry
	if err := c.doJSON(ctx, http.MethodPost, "/api/v1/meal-plans", payload, &out); err != nil {
		return MealPlanEntry{}, err
	}
	return out, nil
}

// DeleteMealPlan removes a recipe from the meal plan on the given date.
func (c *Client) DeleteMealPlan(ctx context.Context, date, recipeID string) error {
	path := fmt.Sprintf("/api/v1/meal-plans/%s/%s", url.PathEscape(date), url.PathEscape(recipeID))
	return c.doJSON(ctx, http.MethodDelete, path, nil, nil)
}

// Recipe returns the full recipe detail by id.
func (c *Client) Recipe(ctx context.Context, id string) (RecipeDetail, error) {
	path := fmt.Sprintf("/api/v1/recipes/%s", id)
	var out RecipeDetail
	if err := c.doJSON(ctx, http.MethodGet, path, nil, &out); err != nil {
		return RecipeDetail{}, err
	}
	return out, nil
}

// CreateRecipe creates a recipe from a raw JSON payload.
func (c *Client) CreateRecipe(ctx context.Context, payload json.RawMessage) (RecipeDetail, error) {
	var out RecipeDetail
	if err := c.doJSON(ctx, http.MethodPost, "/api/v1/recipes", payload, &out); err != nil {
		return RecipeDetail{}, err
	}
	return out, nil
}

// UpdateRecipe updates a recipe by id from a raw JSON payload.
func (c *Client) UpdateRecipe(ctx context.Context, id string, payload json.RawMessage) (RecipeDetail, error) {
	path := fmt.Sprintf("/api/v1/recipes/%s", id)
	var out RecipeDetail
	if err := c.doJSON(ctx, http.MethodPut, path, payload, &out); err != nil {
		return RecipeDetail{}, err
	}
	return out, nil
}

// DeleteRecipe soft-deletes a recipe by id.
func (c *Client) DeleteRecipe(ctx context.Context, id string) error {
	path := fmt.Sprintf("/api/v1/recipes/%s", id)
	return c.doJSON(ctx, http.MethodDelete, path, nil, nil)
}

// RestoreRecipe restores a soft-deleted recipe by id.
func (c *Client) RestoreRecipe(ctx context.Context, id string) error {
	path := fmt.Sprintf("/api/v1/recipes/%s/restore", id)
	return c.doJSON(ctx, http.MethodPut, path, nil, nil)
}

func (c *Client) doJSON(ctx context.Context, method, path string, body interface{}, out interface{}) error {
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

func (c *Client) doJSONWithQuery(ctx context.Context, method, path string, query url.Values, body interface{}, out interface{}) error {
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
	if len(query) > 0 {
		req.URL.RawQuery = query.Encode()
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

func (c *Client) newRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	rel := &url.URL{Path: path}
	u := c.baseURL.ResolveReference(rel)
	req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))
	}
	return req, nil
}

func (c *Client) do(req *http.Request) (*http.Response, error) {
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

func (c *Client) debugf(format string, args ...any) {
	if c.debugWriter == nil {
		return
	}
	if _, err := fmt.Fprintf(c.debugWriter, format, args...); err != nil {
		// Best-effort debug logging only.
		_ = err
	}
}

func (c *Client) closeBody(resp *http.Response) {
	if resp == nil || resp.Body == nil {
		return
	}
	if err := resp.Body.Close(); err != nil && c.debug {
		c.debugf("close body error: %v\n", err)
	}
}

func readAPIError(resp *http.Response) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read error response: %w", err)
	}

	if len(bytes.TrimSpace(body)) == 0 {
		return &APIError{StatusCode: resp.StatusCode}
	}

	var problem Problem
	if err := json.Unmarshal(body, &problem); err != nil {
		return &APIError{StatusCode: resp.StatusCode}
	}

	return &APIError{
		StatusCode: resp.StatusCode,
		Problem:    problem,
	}
}
