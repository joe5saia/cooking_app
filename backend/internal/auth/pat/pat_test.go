package pat

import "testing"

func TestGenerate(t *testing.T) {
	secret, hash, err := Generate()
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}
	if err := ValidateSecret(secret); err != nil {
		t.Fatalf("ValidateSecret error: %v", err)
	}
	if len(hash) != 64 {
		t.Fatalf("hash len=%d, want 64", len(hash))
	}
	if hash != Hash(secret) {
		t.Fatalf("Hash(secret) mismatch")
	}
}
