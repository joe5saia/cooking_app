package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

func (a *App) decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	if w == nil || r == nil {
		return errInternal(errors.New("request and response writer are required"))
	}

	if r.Body == nil {
		return errBadRequest("invalid JSON")
	}

	if a.maxJSONBodyBytes > 0 {
		r.Body = http.MaxBytesReader(w, r.Body, a.maxJSONBodyBytes)
	}

	dec := json.NewDecoder(r.Body)
	if a.strictJSON {
		dec.DisallowUnknownFields()
	}

	if err := dec.Decode(dst); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return errRequestTooLarge()
		}
		return errBadRequest("invalid JSON")
	}

	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errBadRequest("invalid JSON")
	}

	return nil
}
