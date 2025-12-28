package app

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/saiaj/cooking_app/backend/internal/cookctl/config"
	"github.com/saiaj/cooking_app/backend/internal/cookctl/credentials"
)

func TestRunCompletionBash(t *testing.T) {
	t.Parallel()

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

	exitCode := app.runCompletion([]string{"bash"})
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}

	output := stdout.String()
	if !strings.Contains(output, "complete -F _cookctl cookctl") {
		t.Fatalf("expected bash completion output, got %q", output)
	}
	if !strings.Contains(output, "auth") {
		t.Fatalf("expected auth completion entry, got %q", output)
	}
}

func TestRunCompletionZsh(t *testing.T) {
	t.Parallel()

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

	exitCode := app.runCompletion([]string{"zsh"})
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}

	output := stdout.String()
	if !strings.Contains(output, "#compdef cookctl") {
		t.Fatalf("expected zsh completion output, got %q", output)
	}
	if !strings.Contains(output, "completion") {
		t.Fatalf("expected completion entry, got %q", output)
	}
}

func TestRunCompletionFish(t *testing.T) {
	t.Parallel()

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

	exitCode := app.runCompletion([]string{"fish"})
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d", exitCode, exitOK)
	}

	output := stdout.String()
	if !strings.Contains(output, "complete -c cookctl") {
		t.Fatalf("expected fish completion output, got %q", output)
	}
	if !strings.Contains(output, "recipe") {
		t.Fatalf("expected recipe completion entry, got %q", output)
	}
}

func TestRunCompletionInvalidShell(t *testing.T) {
	t.Parallel()

	stderr := &bytes.Buffer{}
	app := &App{
		cfg: config.Config{
			Output:  config.OutputTable,
			Timeout: 5 * time.Second,
		},
		stdin:  bytes.NewBufferString(""),
		stdout: &bytes.Buffer{},
		stderr: stderr,
		store:  credentials.NewStore(filepath.Join(t.TempDir(), "credentials.json")),
	}

	exitCode := app.runCompletion([]string{"powershell"})
	if exitCode != exitUsage {
		t.Fatalf("exit code = %d, want %d", exitCode, exitUsage)
	}
	if !strings.Contains(stderr.String(), "unsupported shell") {
		t.Fatalf("expected unsupported shell error, got %q", stderr.String())
	}
}
