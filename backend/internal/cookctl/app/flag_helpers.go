package app

import (
	"flag"
	"io"
	"sort"
)

type flagSetBuilder func(io.Writer) *flag.FlagSet

// newFlagSet creates a flagset with shared output and usage wiring.
func newFlagSet(name string, out io.Writer, usage func(io.Writer)) *flag.FlagSet {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(out)
	if usage != nil {
		flags.Usage = func() {
			usage(out)
		}
	}
	return flags
}

// printUsageWithFlags prints usage lines and flag defaults from the builder.
func printUsageWithFlags(w io.Writer, lines []string, buildFlags flagSetBuilder) {
	for _, line := range lines {
		writeLine(w, line)
	}
	if buildFlags == nil {
		return
	}
	flags := buildFlags(w)
	if !flagSetHasFlags(flags) {
		return
	}
	writeLine(w, "flags:")
	flags.PrintDefaults()
}

// flagNames returns sorted flag names for completion.
func flagNames(buildFlags flagSetBuilder) []string {
	if buildFlags == nil {
		return nil
	}
	flags := buildFlags(io.Discard)
	names := make([]string, 0, 8)
	flags.VisitAll(func(f *flag.Flag) {
		if len(f.Name) == 1 {
			names = append(names, "-"+f.Name)
			return
		}
		names = append(names, "--"+f.Name)
	})
	sort.Strings(names)
	return names
}

func flagSetHasFlags(flags *flag.FlagSet) bool {
	hasFlags := false
	flags.VisitAll(func(_ *flag.Flag) {
		hasFlags = true
	})
	return hasFlags
}
