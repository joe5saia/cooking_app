package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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

func (a *App) handleUsersList(w http.ResponseWriter, r *http.Request) error {
	if _, ok := authInfoFromRequest(r); !ok {
		return errUnauthorized("unauthorized")
	}

	repo, err := users.New(a.queries)
	if err != nil {
		return errInternal(err)
	}

	rows, err := repo.List(r.Context())
	if err != nil {
		return errInternal(err)
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
	return nil
}

func (a *App) handleUsersCreate(w http.ResponseWriter, r *http.Request) error {
	info, ok := authInfoFromRequest(r)
	if !ok {
		return errUnauthorized("unauthorized")
	}

	var req createUserRequest
	if err := a.decodeJSON(w, r, &req); err != nil {
		return err
	}

	username, err := users.NormalizeUsername(req.Username)
	if err != nil {
		return errValidationField("username", err.Error())
	}
	req.Password = strings.TrimSpace(req.Password)
	if req.Password == "" {
		return errValidationField("password", "password is required")
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
		return errInternal(err)
	}

	repo, err := users.New(a.queries)
	if err != nil {
		return errInternal(err)
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
		if isPGUniqueViolation(err) {
			return errValidationField("username", "username already exists")
		}
		return errInternal(err)
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
	return nil
}

func (a *App) handleUsersDeactivate(w http.ResponseWriter, r *http.Request) error {
	info, ok := authInfoFromRequest(r)
	if !ok {
		return errUnauthorized("unauthorized")
	}

	id, err := parseUUIDParam(r, "id")
	if err != nil {
		return err
	}

	repo, err := users.New(a.queries)
	if err != nil {
		return errInternal(err)
	}

	_, err = repo.SetActive(r.Context(), id, false, info.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errNotFound()
		}
		return errInternal(err)
	}

	w.WriteHeader(http.StatusNoContent)
	return nil
}
