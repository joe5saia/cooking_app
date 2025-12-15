package httpapi

import (
	"context"
	"net/http"

	"github.com/google/uuid"
)

type authType string

const (
	authTypeSession authType = "session"
	authTypePAT     authType = "pat"
)

type authInfo struct {
	UserID   uuid.UUID
	AuthType authType
}

type authInfoKey struct{}

func withAuthInfo(ctx context.Context, info authInfo) context.Context {
	return context.WithValue(ctx, authInfoKey{}, info)
}

func authInfoFromRequest(r *http.Request) (authInfo, bool) {
	if r == nil {
		return authInfo{}, false
	}
	v := r.Context().Value(authInfoKey{})
	info, ok := v.(authInfo)
	return info, ok
}
