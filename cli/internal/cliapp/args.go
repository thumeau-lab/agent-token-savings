package cliapp

import (
	"flag"
	"fmt"

	"github.com/thumeau-lab/agent-token-savings/cli/internal/i18n"
)

// parsePositionalAndFlags parses args for a command whose usage is
// `ats <cmd> [agent] [flags...]`: Go's flag package stops consuming
// flags at the first non-flag argument, so a flag placed after the
// positional agent (the documented order, e.g. `ats cache claude-code
// --days 1`) would otherwise land in fs.Args() and be silently ignored.
//
// It parses in two passes: once for any flags preceding the positional,
// then -- if a positional argument was found and tokens remain after it
// -- a second pass over those remaining tokens, so flags are accepted
// before or after the agent (or both). A second positional argument, or
// an unknown flag in either pass, writes a usage error to fs's output
// (as fs.Parse already does) and returns ok=false; the caller should
// return exit code 2 without touching fs's flag values.
//
// Returns the single positional argument (empty string if none).
func parsePositionalAndFlags(fs *flag.FlagSet, args []string, lang i18n.Lang) (positional string, ok bool) {
	if err := fs.Parse(args); err != nil {
		return "", false
	}
	rest := fs.Args()
	if len(rest) == 0 {
		return "", true
	}
	positional = rest[0]
	remaining := rest[1:]
	if len(remaining) == 0 {
		return positional, true
	}
	if err := fs.Parse(remaining); err != nil {
		return "", false
	}
	if fs.NArg() > 0 {
		fmt.Fprintln(fs.Output(), i18n.T(lang, "err.unexpected_argument", fs.Arg(0)))
		fs.Usage()
		return "", false
	}
	return positional, true
}
