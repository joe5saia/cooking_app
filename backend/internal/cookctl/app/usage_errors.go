package app

import (
	"fmt"
	"io"
)

// usageError writes a usage error message and returns exitUsage.
func usageError(w io.Writer, message string) int {
	writeLine(w, message)
	return exitUsage
}

// usageErrorf writes a formatted usage error message and returns exitUsage.
func usageErrorf(w io.Writer, format string, args ...any) int {
	writeLine(w, fmt.Sprintf(format, args...))
	return exitUsage
}
