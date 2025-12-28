// Package credentials persists cookctl authentication credentials.
package credentials

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/saiaj/cooking_app/backend/internal/cookctl/config"
)

// Credentials contains the stored PAT metadata.
type Credentials struct {
	Token     string     `json:"token"`
	TokenID   string     `json:"token_id,omitempty"`
	TokenName string     `json:"token_name,omitempty"`
	CreatedAt *time.Time `json:"created_at,omitempty"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	APIURL    string     `json:"api_url,omitempty"`
}

// Store reads and writes credentials on disk.
type Store struct {
	path string
}

// NewStore returns a credential store bound to the provided path.
func NewStore(path string) *Store {
	return &Store{path: path}
}

// DefaultPath resolves the OS-specific credentials path.
func DefaultPath() (string, error) {
	dir, err := config.DefaultDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "credentials.json"), nil
}

// Load returns credentials if the file exists.
func (s *Store) Load() (Credentials, bool, error) {
	raw, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Credentials{}, false, nil
		}
		return Credentials{}, false, fmt.Errorf("read credentials: %w", err)
	}

	var creds Credentials
	if err := json.Unmarshal(raw, &creds); err != nil {
		return Credentials{}, false, fmt.Errorf("parse credentials: %w", err)
	}
	if creds.Token == "" {
		return Credentials{}, false, nil
	}
	return creds, true, nil
}

// Save writes credentials to disk with secure permissions where supported.
func (s *Store) Save(creds Credentials) error {
	if creds.Token == "" {
		return errors.New("token is required")
	}
	if err := ensureDir(filepath.Dir(s.path)); err != nil {
		return err
	}

	payload, err := json.Marshal(creds)
	if err != nil {
		return fmt.Errorf("encode credentials: %w", err)
	}
	if err := os.WriteFile(s.path, payload, 0o600); err != nil {
		return fmt.Errorf("write credentials: %w", err)
	}

	if runtime.GOOS != "windows" {
		if err := os.Chmod(s.path, 0o600); err != nil {
			return fmt.Errorf("chmod credentials: %w", err)
		}
	}
	return nil
}

// Clear removes stored credentials if they exist.
func (s *Store) Clear() error {
	if err := os.Remove(s.path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove credentials: %w", err)
	}
	return nil
}

// ensureDir creates the credentials directory with restricted permissions.
func ensureDir(path string) error {
	if err := os.MkdirAll(path, 0o700); err != nil {
		return fmt.Errorf("create credentials dir: %w", err)
	}
	return nil
}
