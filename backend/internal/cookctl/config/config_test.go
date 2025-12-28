package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "missing.toml")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.APIURL != "http://localhost:8080" {
		t.Fatalf("APIURL = %q, want %q", cfg.APIURL, "http://localhost:8080")
	}
	if cfg.Output != OutputTable {
		t.Fatalf("Output = %q, want %q", cfg.Output, OutputTable)
	}
	if cfg.Timeout != 30*time.Second {
		t.Fatalf("Timeout = %s, want %s", cfg.Timeout, 30*time.Second)
	}
}

func TestLoadFromFileAndEnv(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(`api_url = "http://example.test"
output = "json"
timeout = "12s"
debug = true
`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	t.Setenv("COOKING_API_URL", "http://override.test")
	t.Setenv("COOKING_OUTPUT", "table")
	t.Setenv("COOKING_TIMEOUT", "8s")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.APIURL != "http://override.test" {
		t.Fatalf("APIURL = %q, want %q", cfg.APIURL, "http://override.test")
	}
	if cfg.Output != OutputTable {
		t.Fatalf("Output = %q, want %q", cfg.Output, OutputTable)
	}
	if cfg.Timeout != 8*time.Second {
		t.Fatalf("Timeout = %s, want %s", cfg.Timeout, 8*time.Second)
	}
	if cfg.Debug != true {
		t.Fatalf("Debug = %t, want true", cfg.Debug)
	}
}

func TestParseOutputInvalid(t *testing.T) {
	if _, err := ParseOutput("nope"); err == nil {
		t.Fatalf("ParseOutput expected error")
	}
}

func TestSaveAndLoadFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "config.toml")
	cfg := Config{
		APIURL:  "http://example.test",
		Output:  OutputJSON,
		Timeout: 15 * time.Second,
		Debug:   true,
	}
	if err := Save(path, cfg); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	loaded, err := LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile returned error: %v", err)
	}
	if loaded.APIURL != cfg.APIURL {
		t.Fatalf("APIURL = %q, want %q", loaded.APIURL, cfg.APIURL)
	}
	if loaded.Output != cfg.Output {
		t.Fatalf("Output = %q, want %q", loaded.Output, cfg.Output)
	}
	if loaded.Timeout != cfg.Timeout {
		t.Fatalf("Timeout = %s, want %s", loaded.Timeout, cfg.Timeout)
	}
	if loaded.Debug != cfg.Debug {
		t.Fatalf("Debug = %t, want %t", loaded.Debug, cfg.Debug)
	}
}
