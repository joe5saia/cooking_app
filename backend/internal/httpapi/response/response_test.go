package response

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteProblem_SetsStatusAndContentTypeAndBody(t *testing.T) {
	rec := httptest.NewRecorder()

	if err := WriteProblem(rec, http.StatusBadRequest, "bad_request", "bad request", nil); err != nil {
		t.Fatalf("WriteProblem error: %v", err)
	}

	res := rec.Result()
	t.Cleanup(func() {
		if err := res.Body.Close(); err != nil {
			t.Errorf("close body: %v", err)
		}
	})

	if got, want := res.StatusCode, http.StatusBadRequest; got != want {
		t.Fatalf("StatusCode=%d, want %d", got, want)
	}

	if got, want := res.Header.Get("Content-Type"), contentTypeJSON; got != want {
		t.Fatalf("Content-Type=%q, want %q", got, want)
	}

	var body map[string]any
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	if got, want := body["code"], "bad_request"; got != want {
		t.Fatalf("code=%v, want %v", got, want)
	}
	if got, want := body["message"], "bad request"; got != want {
		t.Fatalf("message=%v, want %v", got, want)
	}
	if _, ok := body["details"]; ok {
		t.Fatalf("details unexpectedly present: %v", body["details"])
	}
}

func TestWriteValidationProblem_IncludesDetails(t *testing.T) {
	rec := httptest.NewRecorder()

	var errs ValidationErrors
	errs.Add("title", "is required")

	if err := WriteValidationProblem(rec, errs); err != nil {
		t.Fatalf("WriteValidationProblem error: %v", err)
	}

	res := rec.Result()
	t.Cleanup(func() {
		if err := res.Body.Close(); err != nil {
			t.Errorf("close body: %v", err)
		}
	})

	if got, want := res.StatusCode, http.StatusBadRequest; got != want {
		t.Fatalf("StatusCode=%d, want %d", got, want)
	}

	var p Problem
	if err := json.NewDecoder(res.Body).Decode(&p); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	if got, want := p.Code, "validation_error"; got != want {
		t.Fatalf("code=%q, want %q", got, want)
	}
	if got, want := p.Message, "validation failed"; got != want {
		t.Fatalf("message=%q, want %q", got, want)
	}
	if p.Details == nil {
		t.Fatalf("details unexpectedly nil")
	}
}
