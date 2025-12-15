package pat

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
)

// Prefix is the required PAT secret prefix.
const Prefix = "cooking_app_pat_"

// Generate returns a new PAT secret (Prefix + random) and its sha256 hex hash.
// Only the hash should be stored.
func Generate() (string, string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", "", err
	}

	secret := Prefix + base64.RawURLEncoding.EncodeToString(raw[:])
	hash := Hash(secret)
	return secret, hash, nil
}

// Hash computes the sha256 hex hash of a token secret.
func Hash(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

// ValidateSecret checks basic token formatting before hashing.
func ValidateSecret(secret string) error {
	if secret == "" {
		return errors.New("token is required")
	}
	if !strings.HasPrefix(secret, Prefix) {
		return errors.New("invalid token prefix")
	}
	if len(secret) <= len(Prefix) {
		return errors.New("token is too short")
	}
	return nil
}
