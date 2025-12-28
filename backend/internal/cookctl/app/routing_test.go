package app

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"
)

func TestRunGlobalFlagsAfterCommand(t *testing.T) {
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

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := Run([]string{"cookctl", "version", "--output", "json"}, bytes.NewBufferString(""), stdout, stderr)
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

func TestSplitGlobalArgsInterspersed(t *testing.T) {
	t.Parallel()

	globalArgs, commandArgs, err := splitGlobalArgs([]string{
		"recipe",
		"list",
		"--output",
		"json",
		"--timeout=15s",
		"--debug",
	})
	if err != nil {
		t.Fatalf("split global args: %v", err)
	}

	expectedGlobals := []string{"--output", "json", "--timeout=15s", "--debug"}
	if !reflect.DeepEqual(globalArgs, expectedGlobals) {
		t.Fatalf("global args = %v, want %v", globalArgs, expectedGlobals)
	}

	expectedCommands := []string{"recipe", "list"}
	if !reflect.DeepEqual(commandArgs, expectedCommands) {
		t.Fatalf("command args = %v, want %v", commandArgs, expectedCommands)
	}
}

func TestSplitGlobalArgsHelpAfterCommand(t *testing.T) {
	t.Parallel()

	globalArgs, commandArgs, err := splitGlobalArgs([]string{
		"recipe",
		"list",
		"--help",
		"--output",
		"json",
	})
	if err != nil {
		t.Fatalf("split global args: %v", err)
	}

	expectedGlobals := []string{"--output", "json"}
	if !reflect.DeepEqual(globalArgs, expectedGlobals) {
		t.Fatalf("global args = %v, want %v", globalArgs, expectedGlobals)
	}

	expectedCommands := []string{"recipe", "list", "--help"}
	if !reflect.DeepEqual(commandArgs, expectedCommands) {
		t.Fatalf("command args = %v, want %v", commandArgs, expectedCommands)
	}
}

func TestSplitGlobalArgsMissingValue(t *testing.T) {
	t.Parallel()

	_, _, err := splitGlobalArgs([]string{"recipe", "list", "--output"})
	if err == nil {
		t.Fatalf("expected error for missing flag value")
	}
}
