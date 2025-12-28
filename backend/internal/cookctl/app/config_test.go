package app

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/saiaj/cooking_app/backend/internal/cookctl/config"
	"github.com/saiaj/cooking_app/backend/internal/cookctl/credentials"
)

func TestRunConfigSetAndView(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config.toml")
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	app := &App{
		cfg: config.Config{
			Output:  config.OutputJSON,
			Timeout: 5 * time.Second,
		},
		stdin:  bytes.NewBufferString(""),
		stdout: stdout,
		stderr: stderr,
		store:  credentials.NewStore(filepath.Join(t.TempDir(), "credentials.json")),
	}

	exitCode := app.runConfigSet([]string{
		"--config", configPath,
		"--api-url", "http://example.test",
		"--output", "json",
		"--timeout", "45s",
		"--debug",
	})
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}

	stdout.Reset()
	exitCode = app.runConfigView([]string{"--config", configPath})
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}

	var view configView
	if err := json.NewDecoder(bytes.NewReader(stdout.Bytes())).Decode(&view); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if view.APIURL != "http://example.test" {
		t.Fatalf("api url = %q, want %q", view.APIURL, "http://example.test")
	}
	if view.Output != "json" {
		t.Fatalf("output = %q, want json", view.Output)
	}
	if view.Timeout != "45s" {
		t.Fatalf("timeout = %q, want 45s", view.Timeout)
	}
	if view.Debug != true {
		t.Fatalf("debug = %t, want true", view.Debug)
	}
}

func TestRunConfigUnset(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config.toml")
	if err := config.Save(configPath, config.Config{
		APIURL:  "http://example.test",
		Output:  config.OutputJSON,
		Timeout: 45 * time.Second,
		Debug:   true,
	}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	stdout := &bytes.Buffer{}
	app := &App{
		cfg: config.Config{
			Output:  config.OutputJSON,
			Timeout: 5 * time.Second,
		},
		stdin:  bytes.NewBufferString(""),
		stdout: stdout,
		stderr: &bytes.Buffer{},
		store:  credentials.NewStore(filepath.Join(t.TempDir(), "credentials.json")),
	}

	exitCode := app.runConfigUnset([]string{"--config", configPath, "--api-url", "--debug"})
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}

	var view configView
	if err := json.NewDecoder(bytes.NewReader(stdout.Bytes())).Decode(&view); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if view.APIURL != "http://localhost:8080" {
		t.Fatalf("api url = %q, want default", view.APIURL)
	}
	if view.Debug != false {
		t.Fatalf("debug = %t, want false", view.Debug)
	}
}

func TestRunConfigPath(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}
	app := &App{
		cfg: config.Config{
			Output:  config.OutputJSON,
			Timeout: 5 * time.Second,
		},
		stdin:  bytes.NewBufferString(""),
		stdout: stdout,
		stderr: &bytes.Buffer{},
		store:  credentials.NewStore(filepath.Join(t.TempDir(), "credentials.json")),
	}

	exitCode := app.runConfigPath(nil)
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}

	var view configView
	if err := json.NewDecoder(bytes.NewReader(stdout.Bytes())).Decode(&view); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if view.ConfigPath == "" {
		t.Fatalf("expected config path")
	}
}
