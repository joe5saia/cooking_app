package app

import (
	"bytes"
	"context"
	"errors"
	"testing"
)

func TestRunWritesMessage(t *testing.T) {
	t.Parallel()

	buffer := &bytes.Buffer{}

	err := Run(context.Background(), buffer, Config{AppName: "Cooking"})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	got := buffer.String()
	want := "Cooking: ready\n"
	if got != want {
		t.Fatalf("unexpected output: got %q, want %q", got, want)
	}
}

func TestRunRequiresWriter(t *testing.T) {
	t.Parallel()

	err := Run(context.Background(), nil, Config{AppName: "Cooking"})
	if err == nil {
		t.Fatal("expected error when writer is nil")
	}
}

func TestRunContextCanceled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	buffer := &bytes.Buffer{}
	err := Run(ctx, buffer, Config{AppName: "Cooking"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation error, got %v", err)
	}
}
