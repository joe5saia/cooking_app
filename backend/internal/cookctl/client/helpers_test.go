package client

import (
	"encoding/json"
	"net/http"
	"testing"
)

// writeJSON writes a JSON response and fails the test on error.
func writeJSON(t *testing.T, w http.ResponseWriter, payload interface{}) {
	t.Helper()
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		t.Fatalf("encode response: %v", err)
	}
}
