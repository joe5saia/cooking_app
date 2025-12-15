package httpapi

import (
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

func (a *App) requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

		next.ServeHTTP(ww, r)

		status := ww.Status()
		if status == 0 {
			status = http.StatusOK
		}

		remoteIP := strings.TrimSpace(r.RemoteAddr)
		if host, _, err := net.SplitHostPort(remoteIP); err == nil {
			remoteIP = host
		}

		attrs := []any{
			"request_id", middleware.GetReqID(r.Context()),
			"remote_ip", remoteIP,
			"method", r.Method,
			"path", r.URL.Path,
			"status", status,
			"duration_ms", time.Since(start).Milliseconds(),
		}

		if info, ok := authInfoFromRequest(r); ok {
			attrs = append(attrs,
				"user_id", info.UserID.String(),
				"auth_type", string(info.AuthType),
			)
		}

		a.logger.Info("request", attrs...)
	})
}
