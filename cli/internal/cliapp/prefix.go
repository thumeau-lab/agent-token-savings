package cliapp

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/thumeau-lab/agent-token-savings/cli/internal/agents"
	"github.com/thumeau-lab/agent-token-savings/cli/internal/i18n"
	"github.com/thumeau-lab/agent-token-savings/cli/internal/prefixinfo"
)

// ComponentJSON is one named contributor to an agent's cached prefix.
type ComponentJSON struct {
	Name  string `json:"name"`
	Bytes int64  `json:"bytes"`
}

// AgentPrefixResult is one agent's entry in `ats prefix`'s stable JSON schema.
type AgentPrefixResult struct {
	Agent           string          `json:"agent"`
	Skipped         bool            `json:"skipped"`
	Reason          string          `json:"reason,omitempty"`
	TotalBytes      int64           `json:"total_bytes,omitempty"`
	EstimatedTokens int64           `json:"estimated_tokens,omitempty"`
	TokenEstimate   string          `json:"token_estimate_method,omitempty"`
	Components      []ComponentJSON `json:"components,omitempty"`
	BuiltinTokens   int64           `json:"builtin_tokens,omitempty"`
	BuiltinSource   string          `json:"builtin_source,omitempty"`

	// MCPServerCount/MCPServersNote are set for agents that track named
	// MCP servers (claude-code): the count is always present (0 when
	// none), and the note spells it out in words -- "0 MCP servers
	// configured", or "N MCP servers configured: name1, name2" -- so
	// absence is never silently indistinguishable from "not measured".
	MCPServerCount int    `json:"mcp_server_count"`
	MCPServersNote string `json:"mcp_servers_note,omitempty"`
}

// PrefixPayload is the top-level JSON object printed by `ats prefix --json`.
type PrefixPayload struct {
	Command string              `json:"command"`
	Agents  []AgentPrefixResult `json:"agents"`
}

func prefixSkipped(k agents.Kind, reason string) AgentPrefixResult {
	return AgentPrefixResult{Agent: string(k), Skipped: true, Reason: reason}
}

// componentNameCatalogKeys maps a prefixinfo.Component.Name produced by
// AnalyzeClaudeCode/AnalyzeHermes to its i18n catalog key, for the names
// that are generic labels rather than a file's own name (CLAUDE.md,
// SOUL.md... stay as-is in every language: they are literal filenames,
// not prose). Any Component.Name not in this map passes through
// unchanged.
var componentNameCatalogKeys = map[string]string{
	"skills index":              "component.skills_index",
	"agents index":              "component.agents_index",
	"MCP servers (config.yaml)": "component.mcp_servers_config",
	"toolsets (config.yaml)":    "component.toolsets_config",
}

// localizedComponentName translates a prefixinfo component name through
// componentNameCatalogKeys when it is one of the generic labels, or
// returns it unchanged otherwise (file names, "mcp: <server>"...).
// Called with lang=i18n.EN for --json (see runPrefix's collectLang), so
// the JSON payload's names stay byte-identical to prefixinfo's own
// English constants.
func localizedComponentName(name string, lang i18n.Lang) string {
	if key, ok := componentNameCatalogKeys[name]; ok {
		return i18n.T(lang, key)
	}
	return name
}

func fromReport(rep prefixinfo.Report, lang i18n.Lang) AgentPrefixResult {
	comps := make([]ComponentJSON, 0, len(rep.Components))
	for _, c := range rep.Components {
		comps = append(comps, ComponentJSON{Name: localizedComponentName(c.Name, lang), Bytes: c.Bytes})
	}
	result := AgentPrefixResult{
		Agent: rep.Agent, TotalBytes: rep.TotalBytes, EstimatedTokens: rep.EstimatedTokens(),
		TokenEstimate: "bytes / 4, a rough estimate, not a tokenizer count",
		Components:    comps,
		BuiltinTokens: rep.BuiltinTokens, BuiltinSource: rep.BuiltinSource,
	}
	if rep.MCPServers != nil { // only agents that track named MCP servers set this (claude-code)
		result.MCPServerCount = len(rep.MCPServers)
		if len(rep.MCPServers) == 0 {
			result.MCPServersNote = i18n.T(lang, "note.mcp_zero")
		} else {
			result.MCPServersNote = i18n.T(lang, "note.mcp_count", len(rep.MCPServers), strings.Join(rep.MCPServers, ", "))
		}
	}
	return result
}

func collectPrefix(roots agents.Roots, cwd, agentFilter string, lang i18n.Lang) []AgentPrefixResult {
	var out []AgentPrefixResult
	for _, k := range agents.All {
		if agentFilter != "" && string(k) != agentFilter {
			continue
		}
		switch k {
		case agents.ClaudeCode:
			rep, err := prefixinfo.AnalyzeClaudeCode(roots, cwd)
			if err != nil {
				out = append(out, prefixSkipped(k, err.Error()))
				continue
			}
			// claude-code always has a non-zero BuiltinTokens figure (the
			// built-in system prompt + tool schemas), so it is never fully
			// skipped even with zero local components.
			out = append(out, fromReport(rep, lang))
		case agents.Hermes:
			rep, err := prefixinfo.AnalyzeHermes(roots)
			if err != nil {
				out = append(out, prefixSkipped(k, err.Error()))
				continue
			}
			if len(rep.Components) == 0 {
				out = append(out, prefixSkipped(k, i18n.T(lang, "skip.hermes_no_memory")))
				continue
			}
			out = append(out, fromReport(rep, lang))
		case agents.Codex, agents.Copilot, agents.GeminiCLI:
			out = append(out, prefixSkipped(k, i18n.T(lang, "skip.prefix_not_measured")))
		}
	}
	return out
}

func runPrefix(args []string, stdout, stderr io.Writer, env Env) int {
	lang := resolveLang(env)
	fs := flag.NewFlagSet("prefix", flag.ContinueOnError)
	fs.SetOutput(stderr)
	// --days is accepted (not used: prefix is a static snapshot, not a
	// time-windowed query) so every command accepts --days without
	// erroring, matching cache/ttl's flag.
	fs.Int("days", 7, "ignored by prefix (static snapshot, not time-windowed); accepted for flag parity with cache/ttl")
	jsonOut := fs.Bool("json", false, "print machine-readable JSON instead of styled text")
	agentFilter, ok := parsePositionalAndFlags(fs, args, lang)
	if !ok {
		return 2
	}

	roots, err := agents.DefaultRoots(env.HomeDir)
	if err != nil {
		fmt.Fprintln(stderr, "ats:", err)
		return 1
	}
	cwd := env.WorkDir
	if cwd == "" {
		cwd, err = os.Getwd()
		if err != nil {
			fmt.Fprintln(stderr, "ats:", err)
			return 1
		}
	}

	// --json always stays English/machine format regardless of the UI
	// language: collect with i18n.EN so Reason/MCPServersNote never
	// leak a translated string into the machine-readable payload.
	collectLang := lang
	if *jsonOut {
		collectLang = i18n.EN
	}
	results := collectPrefix(roots, cwd, agentFilter, collectLang)

	if *jsonOut {
		payload := PrefixPayload{Command: "prefix", Agents: results}
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(payload); err != nil {
			fmt.Fprintln(stderr, "ats:", err)
			return 1
		}
		return 0
	}

	renderPrefixText(stdout, env, results)
	return 0
}
