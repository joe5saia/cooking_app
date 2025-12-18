package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"

	"github.com/saiaj/cooking_app/backend/internal/migrations"
)

func main() {
	var (
		migrationsDir = flag.String("migrations", "migrations", "path to goose migrations directory")
		outPath       = flag.String("out", "internal/db/schema.sql", "output schema.sql path")
		check         = flag.Bool("check", false, "fail if -out differs from generated output")
	)
	flag.Parse()

	generated, err := migrations.GenerateSQLCSchemaFromGooseMigrations(*migrationsDir)
	if err != nil {
		fatalf("generate schema: %v", err)
	}

	if *check {
		existing, readErr := os.ReadFile(*outPath)
		if readErr != nil {
			fatalf("read -out: %v", readErr)
		}
		if !bytes.Equal(existing, generated) {
			fatalf("schema drift detected: run `make -C backend schema-generate`")
		}
		return
	}

	if writeErr := os.WriteFile(*outPath, generated, 0o644); writeErr != nil { //nolint:gosec // schema output is not secret material
		fatalf("write -out: %v", writeErr)
	}
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
