package app

import (
	"bytes"
	"testing"
)

func TestUsageError(t *testing.T) {
	t.Parallel()

	buf := &bytes.Buffer{}
	exitCode := usageError(buf, "bad args")
	if exitCode != exitUsage {
		t.Fatalf("exitCode = %d, want %d", exitCode, exitUsage)
	}
	if got := buf.String(); got != "bad args\n" {
		t.Fatalf("message = %q, want %q", got, "bad args\\n")
	}
}

func TestUsageErrorf(t *testing.T) {
	t.Parallel()

	buf := &bytes.Buffer{}
	exitCode := usageErrorf(buf, "bad %s", "flag")
	if exitCode != exitUsage {
		t.Fatalf("exitCode = %d, want %d", exitCode, exitUsage)
	}
	if got := buf.String(); got != "bad flag\n" {
		t.Fatalf("message = %q, want %q", got, "bad flag\\n")
	}
}
