// Package main wires the cookctl CLI entrypoint.
package main

import (
	"os"

	"github.com/saiaj/cooking_app/backend/internal/cookctl/app"
)

func main() {
	os.Exit(app.Run(os.Args, os.Stdin, os.Stdout, os.Stderr))
}
