package httpapi

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5/middleware"
)

// audit writes structured audit events with stable fields.
//
// Audit logs are intended for security and administrative review. They must never
// include secrets such as passwords, session tokens, or personal access tokens.
func (a *App) audit(r *http.Request, event string, attrs ...any) {
	if a == nil || a.logger == nil {
		return
	}

	base := []any{
		"audit", true,
		"event", strings.TrimSpace(event),
	}

	if r != nil {
		base = append(base,
			"request_id", middleware.GetReqID(r.Context()),
			"remote_ip", clientIPKey(r),
			"method", r.Method,
			"path", r.URL.Path,
		)
		if ua := strings.TrimSpace(r.UserAgent()); ua != "" {
			base = append(base, "user_agent", ua)
		}
		if info, ok := authInfoFromRequest(r); ok {
			base = append(base,
				"user_id", info.UserID.String(),
				"auth_type", string(info.AuthType),
			)
		}
	}

	a.logger.Info("audit", append(base, attrs...)...)
}
