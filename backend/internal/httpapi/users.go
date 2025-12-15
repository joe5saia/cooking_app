package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/saiaj/cooking_app/backend/internal/auth/password"
	"github.com/saiaj/cooking_app/backend/internal/auth/users"
	"github.com/saiaj/cooking_app/backend/internal/httpapi/response"
)

type createUserRequest struct {
	Username    string  `json:"username"`
	Password    string  `json:"password"`
	DisplayName *string `json:"display_name"`
}

type userResponse struct {
	ID          string  `json:"id"`
	Username    string  `json:"username"`
	DisplayName *string `json:"display_name"`
	IsActive    bool    `json:"is_active"`
	CreatedAt   string  `json:"created_at"`
}

func (a *App) handleUsersList(w http.ResponseWriter, r *http.Request) {
	if _, ok := authInfoFromRequest(r); !ok {
		a.writeProblem(w, http.StatusUnauthorized, "unauthorized", "unauthorized", nil)
		return
	}

	repo, err := users.New(a.queries)
	if err != nil {
		a.logger.Error("users repo init failed", "err", err)
		a.writeProblem(w, http.StatusInternalServerError, "internal_error", "internal error", nil)
		return
	}

	rows, err := repo.List(r.Context())
	if err != nil {
		a.logger.Error("list users failed", "err", err)
		a.writeProblem(w, http.StatusInternalServerError, "internal_error", "internal error", nil)
		return
	}

	out := make([]userResponse, 0, len(rows))
	for _, row := range rows {
		var displayName *string
		if row.DisplayName.Valid {
			displayName = &row.DisplayName.String
		}
		out = append(out, userResponse{
			ID:          uuidString(row.ID),
			Username:    row.Username,
			DisplayName: displayName,
			IsActive:    row.IsActive,
			CreatedAt:   timeString(row.CreatedAt),
		})
	}

	if err := response.WriteJSON(w, http.StatusOK, out); err != nil {
		a.logger.Warn("write failed", "err", err, "path", "/api/v1/users")
	}
}

func (a *App) handleUsersCreate(w http.ResponseWriter, r *http.Request) {
	info, ok := authInfoFromRequest(r)
	if !ok {
		a.writeProblem(w, http.StatusUnauthorized, "unauthorized", "unauthorized", nil)
		return
	}

	var req createUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		a.writeProblem(w, http.StatusBadRequest, "bad_request", "invalid JSON", nil)
		return
	}

	username, err := users.NormalizeUsername(req.Username)
	if err != nil {
		a.writeValidation(w, "username", err.Error())
		return
	}
	req.Password = strings.TrimSpace(req.Password)
	if req.Password == "" {
		a.writeValidation(w, "password", "password is required")
		return
	}

	var displayName *string
	if req.DisplayName != nil {
		trimmed := strings.TrimSpace(*req.DisplayName)
		if trimmed != "" {
			displayName = &trimmed
		}
	}

	hash, err := password.Hash(req.Password)
	if err != nil {
		a.logger.Error("password hash failed", "err", err)
		a.writeProblem(w, http.StatusInternalServerError, "internal_error", "internal error", nil)
		return
	}

	repo, err := users.New(a.queries)
	if err != nil {
		a.logger.Error("users repo init failed", "err", err)
		a.writeProblem(w, http.StatusInternalServerError, "internal_error", "internal error", nil)
		return
	}

	row, err := repo.Create(r.Context(), users.CreateParams{
		ID:           uuid.New(),
		Username:     username,
		PasswordHash: hash,
		DisplayName:  displayName,
		IsActive:     true,
		CreatedBy:    info.UserID,
		UpdatedBy:    info.UserID,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			a.writeValidation(w, "username", "username already exists")
			return
		}
		a.logger.Error("create user failed", "err", err)
		a.writeProblem(w, http.StatusInternalServerError, "internal_error", "internal error", nil)
		return
	}

	var respDisplayName *string
	if row.DisplayName.Valid {
		respDisplayName = &row.DisplayName.String
	}

	resp := userResponse{
		ID:          uuidString(row.ID),
		Username:    row.Username,
		DisplayName: respDisplayName,
		IsActive:    row.IsActive,
		CreatedAt:   timeString(row.CreatedAt),
	}

	if err := response.WriteJSON(w, http.StatusOK, resp); err != nil {
		a.logger.Warn("write failed", "err", err, "path", "/api/v1/users")
	}
}

func (a *App) handleUsersDeactivate(w http.ResponseWriter, r *http.Request) {
	info, ok := authInfoFromRequest(r)
	if !ok {
		a.writeProblem(w, http.StatusUnauthorized, "unauthorized", "unauthorized", nil)
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		a.writeProblem(w, http.StatusBadRequest, "validation_error", "invalid id", []response.FieldError{
			{Field: "id", Message: "invalid id"},
		})
		return
	}

	repo, err := users.New(a.queries)
	if err != nil {
		a.logger.Error("users repo init failed", "err", err)
		a.writeProblem(w, http.StatusInternalServerError, "internal_error", "internal error", nil)
		return
	}

	_, err = repo.SetActive(r.Context(), id, false, info.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			a.writeProblem(w, http.StatusNotFound, "not_found", "not found", nil)
			return
		}
		a.logger.Error("deactivate user failed", "err", err)
		a.writeProblem(w, http.StatusInternalServerError, "internal_error", "internal error", nil)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
