package app

import (
	"net/http"
	"testing"
)

// writeTestJSON writes a JSON response and fails the test on error.
func writeTestJSON(t *testing.T, w http.ResponseWriter, payload interface{}) {
	t.Helper()
	if err := writeJSON(w, payload); err != nil {
		t.Fatalf("encode response: %v", err)
	}
}

const (
	testVersion          = "v1.2.3"
	testCommit           = "abc123"
	testBuiltAt          = "2025-01-02T03:04:05Z"
	testTokenID          = "token-1"
	testMealPlanDate     = "2025-01-03"
	testRecipeID         = "11111111-1111-1111-1111-111111111111"
	testMealPlanRecipeID = "22222222-2222-2222-2222-222222222222"
)
