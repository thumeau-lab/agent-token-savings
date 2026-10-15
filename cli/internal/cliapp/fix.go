package cliapp

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	huh "charm.land/huh/v2"

	"github.com/thumeau-lab/agent-token-savings/cli/internal/agents"
	"github.com/thumeau-lab/agent-token-savings/cli/internal/fixer"
	"github.com/thumeau-lab/agent-token-savings/cli/internal/i18n"
)

func runFix(args []string, stdout, stderr io.Writer, env Env) int {
	lang := resolveLang(env)
	fs := flag.NewFlagSet("fix", flag.ContinueOnError)
	fs.SetOutput(stderr)
	write := fs.Bool("write", false, "apply the change instead of only describing it")
	// --days is accepted (not used: fix inspects the current config, not
	// a time-windowed query) so every command accepts --days without
	// erroring, matching cache/ttl/prefix's flag.
	fs.Int("days", 7, "ignored by fix; accepted for flag parity with cache/ttl/prefix")
	if _, ok := parsePositionalAndFlags(fs, args, lang); !ok {
		return 2
	}

	roots, err := agents.DefaultRoots(env.HomeDir)
	if err != nil {
		fmt.Fprintln(stderr, "ats:", err)
		return 1
	}
	configPath := filepath.Join(roots.HermesHome, "config.yaml")

	finding, ok, err := fixer.DetectHermesCacheTTLFinding(configPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Fprintln(stdout, i18n.T(lang, "fix.no_config"))
			return 0
		}
		fmt.Fprintln(stderr, "ats:", err)
		return 1
	}
	if !ok {
		fmt.Fprintln(stdout, i18n.T(lang, "fix.nothing_to_fix"))
		return 0
	}

	fmt.Fprintln(stdout, i18n.T(lang, "fix.found_header_cli",
		finding.Agent, finding.Description, finding.Key, finding.OldValue, finding.NewValue, finding.File))

	if !*write {
		if !env.IsTTY {
			fmt.Fprintln(stdout, i18n.T(lang, "fix.dry_run"))
			return 0
		}
		approved, err := confirmFinding(finding, lang)
		if err != nil {
			fmt.Fprintln(stderr, "ats:", err)
			return 1
		}
		if !approved {
			fmt.Fprintln(stdout, i18n.T(lang, "fix.not_applied"))
			return 0
		}
	}

	applied, backup, err := fixer.ApplyFix(finding, true)
	if err != nil {
		fmt.Fprintln(stderr, "ats:", err)
		return 1
	}
	if !applied {
		fmt.Fprintln(stderr, "ats:", i18n.T(lang, "fix.was_not_applied"))
		return 1
	}
	fmt.Fprintln(stdout, i18n.T(lang, "fix.applied", finding.Key, finding.OldValue, finding.NewValue, backup))
	return 0
}

// confirmFinding shows an interactive confirm prompt (huh v2) for one
// finding. Only ever called when the caller has verified stdout is a TTY.
func confirmFinding(f fixer.Finding, lang i18n.Lang) (bool, error) {
	approved := false
	err := huh.NewConfirm().
		Title(i18n.T(lang, "confirm.title", f.Key, f.OldValue, f.NewValue)).
		Description(f.Description).
		Affirmative(i18n.T(lang, "confirm.affirmative")).
		Negative(i18n.T(lang, "confirm.negative")).
		Value(&approved).
		Run()
	return approved, err
}
