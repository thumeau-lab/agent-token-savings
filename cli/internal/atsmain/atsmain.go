// Package atsmain is the single real entry point cli/main.go and
// cli/cmd/ats/main.go both call: it builds the process Env from the
// real terminal/locale state and runs cliapp.Run. Split into its own
// package specifically so `go install .../cli/cmd/ats@latest` (see
// cli/cmd/ats/main.go, the only path that installs a binary literally
// named "ats") never duplicates this logic.
package atsmain

import (
	"os"

	xterm "github.com/charmbracelet/x/term"

	lipgloss "charm.land/lipgloss/v2"

	"github.com/thumeau-lab/agent-token-savings/cli/internal/cliapp"
	"github.com/thumeau-lab/agent-token-savings/cli/internal/render"
)

// Run builds the process Env from the real terminal/locale and runs
// the ats CLI, returning the process exit code (never calls os.Exit
// itself; the caller's main() does that).
func Run() int {
	w, _, err := xterm.GetSize(os.Stdout.Fd())
	if err != nil {
		w = 0
	}
	env := cliapp.Env{
		NoColor:    os.Getenv("NO_COLOR") != "",
		IsTTY:      xterm.IsTerminal(os.Stdout.Fd()),
		StdinIsTTY: xterm.IsTerminal(os.Stdin.Fd()),
		Dark:       lipgloss.HasDarkBackground(os.Stdin, os.Stdout),
		TermWidth:  w,
		ForceASCII: !render.LocaleIsUTF8(os.Getenv),
	}
	return cliapp.Run(os.Args[1:], os.Stdout, os.Stderr, env)
}
