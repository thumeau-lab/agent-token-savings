package cliapp

import (
	"flag"
	"fmt"
	"io"
	"path/filepath"

	"github.com/thumeau-lab/agent-token-savings/cli/internal/agents"
	"github.com/thumeau-lab/agent-token-savings/cli/internal/claudecode"
	"github.com/thumeau-lab/agent-token-savings/cli/internal/fixer"
	"github.com/thumeau-lab/agent-token-savings/cli/internal/hermesusage"
	"github.com/thumeau-lab/agent-token-savings/cli/internal/render"
)

// runDashboard is `ats dashboard`: the non-interactive text
// counterpart to the TUI's Dashboard tab (card: "Same dashboard
// (minus block digits, minus interactivity) for `ats` non-TUI text
// output on a TTY"). It always shows the combined view across every
// detected agent (the TUI's 'a' all/one toggle has no non-interactive
// equivalent); block digits never render here regardless of width,
// since they are an interactive-TUI-only flourish. There is no --json
// for this command yet: no new JSON schema was requested by the card,
// and `ats cache --json`/`ats ttl --json` already expose every number
// this view combines, unchanged.
func runDashboard(args []string, stdout, stderr io.Writer, env Env) int {
	lang := resolveLang(env)
	fs := flag.NewFlagSet("dashboard", flag.ContinueOnError)
	fs.SetOutput(stderr)
	days := fs.Int("days", 7, "size of the look-back window in days")
	if _, ok := parsePositionalAndFlags(fs, args, lang); !ok {
		return 2
	}

	roots, err := agents.DefaultRoots(env.HomeDir)
	if err != nil {
		fmt.Fprintln(stderr, "ats:", err)
		return 1
	}

	now := resolveNow(env)
	cache := collectCache(roots, "", *days, lang, now)
	ttl := collectTTL(roots, "", *days, lang, now)
	skipped := skippedAgentsFor(roots, lang)

	configPath := filepath.Join(roots.HermesHome, "config.yaml")
	finding, hasFinding, ferr := fixer.DetectHermesCacheTTLFinding(configPath)
	if ferr != nil {
		hasFinding = false
	}

	var dailyHermes []hermesusage.DailyRow
	var dailyHermesLong []hermesusage.DailyRow
	if roots.HermesHasData() {
		dailyHermes, _ = hermesusage.QueryDaily(filepath.Join(roots.HermesHome, "state.db"), *days)
		dailyHermesLong, _ = hermesusage.QueryDaily(filepath.Join(roots.HermesHome, "state.db"), dashboardHeatmapLookbackDays)
	}
	var dailyClaude []claudecode.DailyRow
	if roots.ClaudeHasData() {
		// The Activite heatmap wants full history regardless of the
		// selected window (card: "read further back than the window for
		// this panel only"); claudecode.DailyUsage already returns every
		// day, so no separate query is needed the way Hermes needs one.
		dailyClaude, _ = claudecode.DailyUsage(roots.ClaudeHome)
	}

	d := buildDashboard(cache, ttl, finding, hasFinding, dailyHermes, dailyClaude, dailyHermesLong, now, *days, comparisonResult{}, comparisonResult{}, skipped, "", lang)
	r := render.New(render.Options{NoColor: env.NoColor, IsTTY: env.IsTTY, Dark: env.Dark, ForceASCII: env.ForceASCII})
	fmt.Fprintln(stdout, renderDashboard(d, r, cardWidth(env), 0, lang, false))
	return 0
}
