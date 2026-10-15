// Package cliapp wires the ats subcommands: version, cache, prefix, ttl, fix.
package cliapp

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/thumeau-lab/agent-token-savings/cli/internal/i18n"
)

// Version is the ats release version, set at build time via -ldflags on release
// builds (see .goreleaser.yaml); defaults to "dev" for local builds.
var Version = "dev"

// Env carries process-level knobs that commands must not read globally, so
// tests can set them without touching real env vars or the real filesystem.
type Env struct {
	// HomeDir overrides os.UserHomeDir() when non-empty (used by tests and by
	// --agent-root in the future). Left empty, commands fall back to the real home.
	HomeDir string
	// NoColor forces plain output when true (mirrors the NO_COLOR convention).
	NoColor bool
	// IsTTY tells commands whether stdout is a terminal (controls styled vs plain
	// rendering, and whether `ats fix` opens an interactive prompt).
	IsTTY bool
	// WorkDir overrides os.Getwd() when non-empty (used by `ats prefix` to find
	// project-level memory files and MCP config).
	WorkDir string
	// Dark reports whether the terminal has a dark background (set via
	// lipgloss.HasDarkBackground in main.go). Only consulted when IsTTY and
	// NoColor is false; it picks which half of each adaptive card color to use.
	Dark bool
	// TermWidth is the detected terminal column width (0 if unknown), used
	// to size the colored card layout; render_text.go falls back to 80.
	TermWidth int
	// ForceASCII is true when the terminal's locale is not UTF-8 (see
	// render.LocaleIsUTF8, computed once in main.go from LC_ALL/LC_CTYPE/
	// LANG): forces Bar back to plain '#'/'-' glyphs even on a color TTY.
	ForceASCII bool
	// StdinIsTTY tells Run whether stdin is a terminal. Interactive mode
	// (bare `ats`, no args) only starts when both StdinIsTTY and IsTTY are
	// true; piped/redirected input or output falls back to `ats cache`,
	// same as before the TUI existed.
	StdinIsTTY bool
	// Lang selects the UI language (i18n.EN/i18n.FR). Left at its zero
	// value, resolveLang treats it as unset and falls back to
	// i18n.DetectLang (ATS_LANG env, then locale) -- so every existing
	// Env{} literal that predates Lang keeps rendering English exactly
	// as before.
	Lang i18n.Lang
	// Now overrides time.Now() for every command and for the
	// interactive TUI's own model clock (newTUIModel sets tuiModel.now
	// from resolveNow(env), so loadCmd/dashboard goldens inherit it
	// too -- see resolveNow): left nil, everything falls back to the
	// real wall clock exactly as before. Tests set it to a fixed clock
	// so a command's or golden's output never depends on the moment it
	// happens to run (card: "inject a fixed now into every dashboard
	// builder/loader used by ANY golden").
	Now func() time.Time
}

// resolveLang returns env.Lang when set, otherwise detects it from
// ATS_LANG/LC_ALL/LC_MESSAGES/LANG via i18n.DetectLang (main.go only
// ever sets env.Lang from the already-detected value, but tests and
// other callers may leave it unset).
func resolveLang(env Env) i18n.Lang {
	if env.Lang != "" {
		return env.Lang
	}
	return i18n.DetectLang("", os.Getenv)
}

// resolveNow returns env.Now() when set, otherwise the real time.Now():
// the single place every non-interactive command reads "now" from, so
// a test can pin it without touching the real system clock (see Env.Now).
func resolveNow(env Env) time.Time {
	if env.Now != nil {
		return env.Now()
	}
	return time.Now()
}

// extractLangFlag pulls a leading "--lang <value>" or "--lang=<value>"
// token pair out of args (same style as --no-tui below: a global flag
// that must be recognized before the subcommand name, since each
// subcommand's own flag.FlagSet does not know about it). Returns the
// remaining args and the detected language, or ok=false if args named a
// malformed --lang (no value).
func extractLangFlag(args []string) (rest []string, lang i18n.Lang, ok bool) {
	if len(args) == 0 {
		return args, "", true
	}
	first := args[0]
	switch {
	case first == "--lang":
		if len(args) < 2 {
			return nil, "", false
		}
		return args[2:], i18n.DetectLang(args[1], os.Getenv), true
	case strings.HasPrefix(first, "--lang="):
		return args[1:], i18n.DetectLang(strings.TrimPrefix(first, "--lang="), os.Getenv), true
	default:
		return args, "", true
	}
}

// removeAll returns args with every exact occurrence of tok removed,
// preserving the relative order of the rest.
func removeAll(args []string, tok string) []string {
	out := args[:0:0]
	for _, a := range args {
		if a != tok {
			out = append(out, a)
		}
	}
	return out
}

// Run executes the ats CLI for the given args (excluding argv[0]) and returns
// the process exit code. It never calls os.Exit itself, so it stays testable.
func Run(args []string, stdout, stderr io.Writer, env Env) int {
	forceNoTUI := false
	// Leading global flags (--no-tui, --lang) are recognized in either
	// order, before the subcommand name: each subcommand's own
	// flag.FlagSet only knows its own flags, so these two are peeled off
	// here first.
	for {
		switch {
		case len(args) > 0 && args[0] == "--no-tui":
			forceNoTUI = true
			args = args[1:]
		case len(args) > 0 && (args[0] == "--lang" || strings.HasPrefix(args[0], "--lang=")):
			rest, lang, ok := extractLangFlag(args)
			if !ok {
				fmt.Fprintln(stderr, "ats: --lang requires a value (en or fr)")
				return 2
			}
			env.Lang = lang
			args = rest
		default:
			goto flagsDone
		}
	}
flagsDone:
	// --no-tui only decides the bare `ats` TUI-vs-cache fallback; once a
	// subcommand is named explicitly (or after this point regardless of
	// position) it has nothing left to do, but a subcommand's own
	// flag.FlagSet does not know it, so strip every remaining occurrence
	// instead of letting it error as an unknown flag.
	args = removeAll(args, "--no-tui")
	lang := resolveLang(env)
	env.Lang = lang
	// `ats --help`/`-h`/`help` and `ats <cmd> --help`/`-h` (card: both
	// must print the full usage to stdout and exit 0, instead of the
	// top-level falling into "unknown command" and the subcommand form
	// falling into its own flag.FlagSet's one-line stderr usage with
	// exit 2). Same usage text either way -- there is no per-command
	// help page.
	if isHelpRequest(args) {
		fmt.Fprint(stdout, i18n.T(lang, "usage.text"))
		return 0
	}
	if len(args) == 0 {
		if !forceNoTUI && env.IsTTY && env.StdinIsTTY {
			return runTUI(stdout, stderr, env)
		}
		args = []string{"cache"}
	}
	switch args[0] {
	case "version":
		fmt.Fprintf(stdout, "ats %s\n", Version)
		return 0
	case "cache":
		return runCache(args[1:], stdout, stderr, env)
	case "dashboard":
		return runDashboard(args[1:], stdout, stderr, env)
	case "prefix":
		return runPrefix(args[1:], stdout, stderr, env)
	case "ttl":
		return runTTL(args[1:], stdout, stderr, env)
	case "fix":
		return runFix(args[1:], stdout, stderr, env)
	default:
		fmt.Fprintln(stderr, i18n.T(lang, "err.unknown_command", args[0]))
		return 2
	}
}

// knownCommands lists every subcommand name Run's switch recognizes,
// shared with isHelpRequest so `ats <cmd> --help` is only special-cased
// for a real command name (anything else keeps falling through to the
// normal "unknown command" error).
var knownCommands = []string{"version", "cache", "dashboard", "prefix", "ttl", "fix"}

// isHelpRequest reports whether args asks for the top-level usage:
// `ats --help`/`-h`/`help` on their own, or `ats <cmd> --help`/`-h`
// right after a known command name.
func isHelpRequest(args []string) bool {
	if len(args) == 0 {
		return false
	}
	if args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		return true
	}
	if len(args) < 2 || (args[1] != "--help" && args[1] != "-h") {
		return false
	}
	for _, c := range knownCommands {
		if args[0] == c {
			return true
		}
	}
	return false
}
