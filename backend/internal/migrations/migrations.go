package migrations

import (
	"path/filepath"
	"runtime"
)

// Dir returns the absolute path to the repository's `backend/migrations` directory.
func Dir() string {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return "migrations"
	}

	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", "migrations"))
}
