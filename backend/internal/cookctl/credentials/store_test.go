package credentials

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestStoreSaveLoadClear(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "credentials.json")
	store := NewStore(path)

	if err := store.Save(Credentials{Token: "pat_123"}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	creds, ok, err := store.Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if !ok {
		t.Fatalf("Load returned ok=false")
	}
	if creds.Token != "pat_123" {
		t.Fatalf("Token = %q, want %q", creds.Token, "pat_123")
	}

	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("Stat returned error: %v", err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("permissions = %v, want 0600", info.Mode().Perm())
		}
	}

	if err := store.Clear(); err != nil {
		t.Fatalf("Clear returned error: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected credentials to be removed")
	}
}

func TestStoreSaveRequiresToken(t *testing.T) {
	t.Parallel()

	store := NewStore(filepath.Join(t.TempDir(), "credentials.json"))
	if err := store.Save(Credentials{}); err == nil {
		t.Fatalf("Save expected error for empty token")
	}
}
