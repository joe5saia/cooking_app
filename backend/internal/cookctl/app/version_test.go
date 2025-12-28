package app

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/saiaj/cooking_app/backend/internal/cookctl/config"
	"github.com/saiaj/cooking_app/backend/internal/cookctl/credentials"
)

func TestRunVersionJSON(t *testing.T) {
	originalVersion := Version
	originalCommit := Commit
	originalBuiltAt := BuiltAt
	Version = testVersion
	Commit = testCommit
	BuiltAt = testBuiltAt
	t.Cleanup(func() {
		Version = originalVersion
		Commit = originalCommit
		BuiltAt = originalBuiltAt
	})

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

	exitCode := app.runVersion(nil)
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}

	var info versionInfo
	if err := json.NewDecoder(bytes.NewReader(stdout.Bytes())).Decode(&info); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if info.Version != testVersion {
		t.Fatalf("version = %q, want %q", info.Version, testVersion)
	}
	if info.Commit != testCommit {
		t.Fatalf("commit = %q, want %q", info.Commit, testCommit)
	}
	if info.BuiltAt != testBuiltAt {
		t.Fatalf("built_at = %q, want %q", info.BuiltAt, testBuiltAt)
	}
}

func TestRunVersionTable(t *testing.T) {
	originalVersion := Version
	originalCommit := Commit
	originalBuiltAt := BuiltAt
	Version = testVersion
	Commit = testCommit
	BuiltAt = testBuiltAt
	t.Cleanup(func() {
		Version = originalVersion
		Commit = originalCommit
		BuiltAt = originalBuiltAt
	})

	stdout := &bytes.Buffer{}
	app := &App{
		cfg: config.Config{
			Output:  config.OutputTable,
			Timeout: 5 * time.Second,
		},
		stdin:  bytes.NewBufferString(""),
		stdout: stdout,
		stderr: &bytes.Buffer{},
		store:  credentials.NewStore(filepath.Join(t.TempDir(), "credentials.json")),
	}

	exitCode := app.runVersion(nil)
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}

	output := stdout.String()
	if !strings.Contains(output, "version") || !strings.Contains(output, testVersion) {
		t.Fatalf("expected version output, got %q", output)
	}
	if !strings.Contains(output, "commit") || !strings.Contains(output, testCommit) {
		t.Fatalf("expected commit output, got %q", output)
	}
	if !strings.Contains(output, "built_at") || !strings.Contains(output, testBuiltAt) {
		t.Fatalf("expected built_at output, got %q", output)
	}
}

func TestRunVersionFlag(t *testing.T) {
	originalVersion := Version
	originalCommit := Commit
	originalBuiltAt := BuiltAt
	Version = testVersion
	Commit = testCommit
	BuiltAt = testBuiltAt
	t.Cleanup(func() {
		Version = originalVersion
		Commit = originalCommit
		BuiltAt = originalBuiltAt
	})

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("COOKING_OUTPUT", "json")

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := Run([]string{"cookctl", "--version"}, bytes.NewBufferString(""), stdout, stderr)
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}

	var info versionInfo
	if err := json.NewDecoder(bytes.NewReader(stdout.Bytes())).Decode(&info); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if info.Version != testVersion {
		t.Fatalf("version = %q, want %q", info.Version, testVersion)
	}
	if info.Commit != testCommit {
		t.Fatalf("commit = %q, want %q", info.Commit, testCommit)
	}
	if info.BuiltAt != testBuiltAt {
		t.Fatalf("built_at = %q, want %q", info.BuiltAt, testBuiltAt)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}
}
