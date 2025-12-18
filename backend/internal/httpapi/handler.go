package httpapi

import "net/http"

type handlerFunc func(http.ResponseWriter, *http.Request) error

func (a *App) handle(fn handlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := fn(w, r); err != nil {
			a.writeError(w, r, err)
		}
	}
}

func (a *App) writeError(w http.ResponseWriter, r *http.Request, err error) {
	if err == nil {
		return
	}

	if apiErr, ok := asAPIError(err); ok {
		if apiErr.kind == apiErrorInternalError && apiErr.cause != nil {
			a.logger.Error("request failed", "err", apiErr.cause, "path", safePath(r))
		}
		a.writeProblem(w, statusForAPIErrorKind(apiErr.kind), string(apiErr.kind), apiErr.message, apiErr.details)
		return
	}

	a.logger.Error("request failed", "err", err, "path", safePath(r))
	a.writeProblem(w, http.StatusInternalServerError, "internal_error", "internal error", nil)
}

func safePath(r *http.Request) string {
	if r == nil || r.URL == nil {
		return ""
	}
	return r.URL.Path
}
