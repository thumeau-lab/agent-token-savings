package i18n

import "fmt"

// catalog is the single source of every user-facing string ats shows:
// TUI chrome, card labels/verdicts/recommendations, the plain-text
// (--no-tui/piped) renderers, and errors/usage. --json output never goes
// through this catalog: it stays English/machine format in every
// language. Numbers embedded in a template arrive pre-formatted by the
// caller (via render.FormatCountLang/FormatUSDLang/FormatPercentLang) so
// the decimal separator matches the selected language; the catalog only
// owns words.
var catalog = map[string]map[Lang]string{
	// TUI chrome: header, footer, selectors.
	"tui.loading":           {EN: "loading...", FR: "chargement..."},
	"tui.window":            {EN: "window: %dd", FR: "fenêtre : %d j"},
	"tui.no_agents":         {EN: "no agents detected", FR: "aucun agent détecté"},
	"tui.no_agents_locally": {EN: "no agents detected locally", FR: "aucun agent détecté localement"},
	"tui.agent_label":       {EN: "agent: < %s >  (%d/%d)", FR: "agent : < %s >  (%d/%d)"},
	"tui.skipped_prefix":    {EN: "skipped: %s", FR: "ignorés : %s"},
	"tui.no_data":           {EN: "%s: no data", FR: "%s : aucune donnée"},
	"tui.updated_ago":       {EN: "updated %s ago", FR: "mis à jour il y a %s"},
	"tui.header_title":      {EN: "ats %s", FR: "ats %s"},

	// Tab names (btop-style: key digit shown next to the name by the
	// caller). Plain-words rename (owner verdict: "horrible, on comprend
	// pas, n'importe qui doit pouvoir comprendre"): a label a first-time
	// user has never seen before beats an accurate technical term.
	"tab.dashboard": {EN: "Dashboard", FR: "Tableau de bord"},
	"tab.cache":     {EN: "Costs", FR: "Coûts"},
	"tab.prefix":    {EN: "What gets sent", FR: "Contenu envoyé"},
	"tab.ttl":       {EN: "Cache lifetime", FR: "Durée du cache"},
	"tab.fix":       {EN: "Fix", FR: "Corriger"},

	// Footer key hints.
	"key.switch_tab":   {EN: "switch view", FR: "changer de vue"},
	"key.switch_agent": {EN: "agent", FR: "agent"},
	"key.period":       {EN: "period", FR: "période"},
	"key.toggle_scope": {EN: "all/one agent", FR: "tous/un agent"},
	"key.reload":       {EN: "reload", FR: "recharger"},
	"key.scroll":       {EN: "scroll", FR: "défiler"},
	"key.confirm":      {EN: "apply fix", FR: "appliquer le correctif"},
	"key.help":         {EN: "full help", FR: "aide complète"},
	"key.quit":         {EN: "quit", FR: "quitter"},
	"key.lang":         {EN: "language", FR: "langue"},

	// Card subtitle/stat labels (ats cache/prefix/ttl, TTY and TUI).
	"card.last_days":              {EN: "last %d days", FR: "%d derniers jours"},
	"stat.sessions":               {EN: "Sessions", FR: "Sessions"},
	"stat.calls":                  {EN: "Calls", FR: "Appels"},
	"stat.cache_write":            {EN: "Cache write", FR: "Écriture cache"},
	"stat.cache_read":             {EN: "Cache read", FR: "Lecture cache"},
	"stat.input":                  {EN: "Input", FR: "Entrée"},
	"stat.output":                 {EN: "Output", FR: "Sortie"},
	"stat.hit_rate":               {EN: "Hit rate", FR: "Taux de cache"},
	"stat.api_equiv_cost":         {EN: "API-equivalent cost", FR: "Coût équivalent API"},
	"stat.local_total":            {EN: "Local total", FR: "Total local"},
	"stat.estimated_tok":          {EN: "Estimated tok", FR: "Tok estimés"},
	"stat.builtin_tok":            {EN: "Built-in tok", FR: "tokens intégrés"},
	"stat.mcp_servers":            {EN: "MCP servers", FR: "Serveurs MCP"},
	"stat.gap_5min":               {EN: "Gap > 5min", FR: "Écart > 5 min"},
	"stat.writes":                 {EN: "Writes", FR: "Écritures"},
	"stat.of_total":               {EN: "Of total", FR: "Du total"},
	"stat.first_call_writes":      {EN: "First-call writes", FR: "Écritures 1er appel"},
	"stat.mid_gap_writes":         {EN: "Mid-gap writes (5-60m)", FR: "Écritures écart moyen (5-60 min)"},
	"stat.long_gap_writes":        {EN: "Long-gap writes (>1h)", FR: "Écritures écart long (>1 h)"},
	"stat.sessions_without_usage": {EN: "Sessions without usage", FR: "Sessions sans usage"},

	// Bar labels.
	"bar.hit_rate":     {EN: "hit rate", FR: "taux de cache"},
	"bar.write_usd":    {EN: "write $", FR: "écriture $"},
	"bar.read_usd":     {EN: "read $", FR: "lecture $"},
	"bar.input_usd":    {EN: "input $", FR: "entrée $"},
	"bar.output_usd":   {EN: "output $", FR: "sortie $"},
	"bar.no_gap_share": {EN: "no-gap share", FR: "part sans écart"},

	// Cache panel sparkline axis labels (min/max on the right, first/last
	// day under the graph): see tui.go's cacheGraph.
	"graph.min": {EN: "min", FR: "min"},
	"graph.max": {EN: "max", FR: "max"},

	// prefixinfo.Report component names shown as card stat/bar labels
	// (prefix.go's fromReport localizes rep.Components through these;
	// --json always collects with i18n.EN, so the JSON payload's names
	// stay byte-identical to prefixinfo's own English constants).
	"component.skills_index":       {EN: "skills index", FR: "index des skills"},
	"component.agents_index":       {EN: "agents index", FR: "index des agents"},
	"component.mcp_servers_config": {EN: "MCP servers (config.yaml)", FR: "Serveurs MCP (config.yaml)"},
	"component.toolsets_config":    {EN: "toolsets (config.yaml)", FR: "ensembles d'outils (config.yaml)"},

	// Claude Code's measured built-in (not locally measurable) prefix
	// citation, shown in the Prefix verdict when the built-in figure
	// outweighs what ats measured locally (see render_text.go's
	// prefixVerdict). args: default prefix tok, no-tools prefix tok,
	// tool-schemas share of the prefix (a plain integer percent).
	"prefix.claude_builtin_citation": {
		EN: `bench/claude_code_levers.py, docs/levers.md: default prefix %s tok vs %s tok with --tools "" (tool schemas = %s%% of the prefix)`,
		FR: `bench/claude_code_levers.py, docs/levers.md : préfixe par défaut %s tok contre %s tok avec --tools "" (schémas d'outils = %s %% du préfixe)`,
	},

	// Table (worst cache breaks).
	"table.worst_cache_breaks":      {EN: "worst cache breaks", FR: "pires ruptures de cache"},
	"table.header_session":          {EN: "session", FR: "session"},
	"table.header_time":             {EN: "time", FR: "heure"},
	"table.header_tokens_rewritten": {EN: "tokens rewritten", FR: "tokens réécrits"},

	// Table (usage by model: codex/copilot cache card).
	"table.usage_by_model": {EN: "usage by model", FR: "usage par modèle"},
	"table.header_model":   {EN: "model", FR: "modèle"},
	"table.header_input":   {EN: "input", FR: "entrée"},
	"table.header_output":  {EN: "output", FR: "sortie"},
	"table.header_read":    {EN: "read", FR: "lecture"},
	"table.header_write":   {EN: "write", FR: "écriture"},

	// cacheVerdict (one-line plain-words verdict).
	"verdict.cache_write_rebuilt_one": {
		EN: "Cache writes are %s of the API-equivalent cost: %d session rebuilt a warm cache.",
		FR: "Les écritures de cache représentent %s du coût équivalent API : %d session a reconstruit un cache chaud.",
	},
	"verdict.cache_write_rebuilt_many": {
		EN: "Cache writes are %s of the API-equivalent cost: %d sessions rebuilt a warm cache.",
		FR: "Les écritures de cache représentent %s du coût équivalent API : %d sessions ont reconstruit un cache chaud.",
	},
	"verdict.cache_write_no_rebuild": {
		EN: "Cache writes are %s of the API-equivalent cost across %d sessions, no abnormal rebuilds found.",
		FR: "Les écritures de cache représentent %s du coût équivalent API sur %d sessions, aucune reconstruction anormale détectée.",
	},
	"verdict.hit_rate_only": {
		EN: "Cache hit rate is %s over %d calls.",
		FR: "Le taux de cache est de %s sur %d appels.",
	},

	// prefixVerdict.
	"verdict.builtin_outweighs": {
		EN: "Built-in tool schemas and system prompt (~%s tok, not measurable offline) outweigh the %s tok measured locally (%s).",
		FR: "Les schémas d'outils natifs et le prompt système (~%s tok, non mesurables hors ligne) dépassent les %s tok mesurés localement (%s).",
	},
	"verdict.estimated_local_prefix": {
		EN: "Estimated local prefix: ~%s tok.",
		FR: "Préfixe local estimé : ~%s tok.",
	},
	"verdict.component_share": {
		EN: "%s is %s of the %s tok measured locally.",
		FR: "%s représente %s du préfixe de %s tok mesuré localement.",
	},

	// ttl: claude-code recommendation/headline.
	"ttl.cc_rec_switch": {
		EN: "set CLAUDE_CODE_PROMPT_CACHE_TTL=1h (or promptCacheTtl: \"1h\" on v2.1.242+): %d of %d calls land in the 5-60min gap window where the default 5m TTL forces an avoidable rewrite",
		FR: "régler CLAUDE_CODE_PROMPT_CACHE_TTL=1h (ou promptCacheTtl: \"1h\" sur v2.1.242+) : %d appels sur %d tombent dans la fenêtre d'écart 5-60 min, où le TTL par défaut de 5 min force une réécriture évitable",
	},
	"ttl.cc_headline_switch": {
		EN: "Switching to a 1h cache TTL would save an estimated %s of cache-write cost. %s",
		FR: "Passer à un TTL de cache de 1 h économiserait environ %s du coût d'écriture de cache. %s",
	},
	"ttl.cc_rec_keep": {
		EN: "leave CLAUDE_CODE_PROMPT_CACHE_TTL unset (default 5m TTL): the calls with a gap over 1h already force a rewrite either way, and paying the pricier 1h-write multiplier on them is not worth the 5-60min rewrites it would avoid",
		FR: "laisser CLAUDE_CODE_PROMPT_CACHE_TTL non défini (TTL par défaut 5 min) : les appels avec un écart de plus d'1 h forcent déjà une réécriture de toute façon, et payer le multiplicateur d'écriture 1 h plus cher dessus ne vaut pas les réécritures 5-60 min que cela éviterait",
	},
	"ttl.cc_headline_keep": {
		EN: "Keeping the default 5m cache TTL is cheaper here (switching to 1h would cost an estimated %s more). %s",
		FR: "Garder le TTL de cache par défaut de 5 min est moins cher ici (passer à 1 h coûterait environ %s de plus). %s",
	},
	"ttl.hermes_auto_on": {
		EN: "auto TTL is on: it avoids an estimated %s of writes",
		FR: "le TTL auto est actif : il évite environ %s des écritures",
	},
	"ttl.hermes_rec_set_auto": {
		EN: "set prompt_caching.cache_ttl to auto in ~/.hermes/config.yaml: run `ats fix` for the exact change",
		FR: "régler prompt_caching.cache_ttl sur auto dans ~/.hermes/config.yaml : lancer `ats fix` pour le changement exact",
	},
	"ttl.hermes_headline_switch": {
		EN: "Switching to auto TTL saves an estimated %s of writes. %s",
		FR: "Passer au TTL auto économise environ %s des écritures. %s",
	},
	"ttl.hermes_rec_keep_1h": {
		EN: "keep prompt_caching.cache_ttl at 1h: no machine session goes idle long enough to lose the cache",
		FR: "garder prompt_caching.cache_ttl à 1h : aucune session machine ne reste inactive assez longtemps pour perdre le cache",
	},

	// Plain-text renderers (--no-tui, piped, non-TTY output).
	"text.cache_summary":         {EN: "%s: %d sessions, %d calls over %d days", FR: "%s : %d sessions, %d appels sur %d jours"},
	"text.cache_tokens":          {EN: "  cache write %12d  read %12d  input %12d  output %12d", FR: "  écriture %12d  lecture %12d  entrée %12d  sortie %12d"},
	"text.hit_rate_line":         {EN: "  hit rate %s %s", FR: "  taux de cache %s %s"},
	"text.warm_rebuilds":         {EN: "  warm rebuilds (abnormal cache breaks): %d", FR: "  reconstructions à chaud (ruptures de cache anormales) : %d"},
	"text.warm_detail":           {EN: "    %-20s %-24s %12d tokens rewritten", FR: "    %-20s %-24s %12d tokens réécrits"},
	"text.usd_note":              {EN: "  note: %s", FR: "  remarque : %s"},
	"text.usd_share_line":        {EN: "  API-equivalent cost share %-11s %s %s", FR: "  part du coût équivalent API %-11s %s %s"},
	"text.cache_tokens_write_na": {EN: "  cache write n/a  read %12d  input %12d  output %12d", FR: "  écriture cache n/d  lecture %12d  entrée %12d  sortie %12d"},
	"text.sessions_without_usage": {
		EN: "  sessions without usage: %d (interactive-only, no token_count emitted: openai/codex#9660)",
		FR: "  sessions sans usage : %d (interactif uniquement, aucun token_count émis : openai/codex#9660)",
	},
	"text.cost_unknown":   {EN: "  API-equivalent cost: unknown price", FR: "  coût équivalent API : prix inconnu"},
	"text.unknown_price":  {EN: "unknown price", FR: "prix inconnu"},
	"text.not_applicable": {EN: "n/a", FR: "n/d"},
	"text.model_breakdown_row": {
		EN: "  %-20s input %10d output %10d read %10d write %10d hit rate %s",
		FR: "  %-20s entrée %10d sortie %10d lecture %10d écriture %10d taux de cache %s",
	},

	"text.prefix_summary":   {EN: "%s: prefix estimate %d bytes (~%d tokens, %s)", FR: "%s : préfixe estimé à %d octets (~%d tokens, %s)"},
	"text.prefix_component": {EN: "  %-26s %s %8d B", FR: "  %-26s %s %8d o"},
	"text.prefix_builtin":   {EN: "  %-26s built-in (not measurable offline): ~%d tok (%s)", FR: "  %-26s natif (non mesurable hors ligne) : ~%d tok (%s)"},
	"text.prefix_mcp_note":  {EN: "  %s", FR: "  %s"},

	"text.ttl_summary":        {EN: "%s: last %d days", FR: "%s : %d derniers jours"},
	"text.ttl_note":           {EN: "  note: %s", FR: "  remarque : %s"},
	"text.ttl_cc_detail":      {EN: "  %d sessions, %d calls (%d mid-gap 5-60min, %d long-gap >1h)", FR: "  %d sessions, %d appels (%d écart moyen 5-60 min, %d écart long >1 h)"},
	"text.ttl_hermes_detail":  {EN: "  %d machine sessions, %d with a gap > 5min", FR: "  %d sessions machine, %d avec un écart > 5 min"},
	"text.ttl_write_share":    {EN: "  machine writes %d of %d total tokens", FR: "  les sessions machine écrivent %d tokens sur %d au total"},
	"text.ttl_headline":       {EN: "  %s", FR: "  %s"},
	"text.ttl_recommendation": {EN: "  recommendation: %s", FR: "  recommandation : %s"},

	"text.skip_piped": {EN: "%s: skipped (%s)", FR: "%s : ignoré (%s)"},
	"text.skip_tty":   {EN: "%s: no usable local data", FR: "%s : aucune donnée locale utilisable"},

	// Skip reasons / notes shared by cache/prefix/ttl and the TUI selector.
	"skip.no_local_data": {EN: "no local usage data found", FR: "aucune donnée d'usage locale trouvée"},
	"skip.codex": {
		EN: "~/.codex/sessions token_count events are not reliably emitted for interactive sessions upstream (openai/codex#9660), so there is no stable schema to parse yet",
		FR: "~/.codex/sessions : les événements token_count ne sont pas émis de manière fiable pour les sessions interactives en amont (openai/codex#9660), il n'y a donc pas encore de schéma stable à analyser",
	},
	"skip.gemini": {
		EN: "detected but not parsed yet (see README limits)",
		FR: "détecté mais pas encore analysé (voir les limites du README)",
	},
	"skip.ttl_not_applicable": {
		EN: "the 5m/1h TTL lever is a CLI/env setting on this agent, not something ats measures from local data (see docs/levers.md)",
		FR: "le levier TTL 5 min/1 h est un réglage CLI/env sur cet agent, pas quelque chose qu'ats mesure depuis les données locales (voir docs/levers.md)",
	},
	"skip.prefix_not_measured": {
		EN: "prefix composition not measured for this agent yet (see README limits)",
		FR: "la composition du préfixe n'est pas encore mesurée pour cet agent (voir les limites du README)",
	},
	"skip.hermes_no_memory": {
		EN: "no local memory file, skill or config.yaml found",
		FR: "aucun fichier de mémoire local, skill ou config.yaml trouvé",
	},
	"note.claude_usd": {
		EN: "Claude Code on a subscription plan is not billed per token; this is the API-equivalent cost at list price (docs/measurements.md), for comparison only.",
		FR: "Claude Code sur un abonnement n'est pas facturé au token ; ceci est le coût équivalent API au tarif catalogue (docs/measurements.md), à titre de comparaison seulement.",
	},
	"note.claude_method": {
		EN: "Claude Code sessions are fully interactive -- there is no machine/cron/subagent session type the way Hermes' sessions table has, so tools/ttl_auto_estimate.py's machine-session split does not apply here. Computed instead directly from the gap preceding each call: a cache hit under both TTLs (<=5min), a hit only under 1h (5-60min, forces a rewrite under 5m), or a miss under either TTL (>1h).",
		FR: "Les sessions Claude Code sont entièrement interactives -- il n'y a pas de type de session machine/cron/sous-agent comme dans la table sessions de Hermes, donc la séparation par session machine de tools/ttl_auto_estimate.py ne s'applique pas ici. Calculé à la place directement depuis l'écart précédant chaque appel : un hit de cache sous les deux TTL (<=5 min), un hit seulement sous 1 h (5-60 min, force une réécriture sous 5 min), ou un miss sous les deux TTL (>1 h).",
	},
	"note.mcp_zero":  {EN: "0 MCP servers configured", FR: "0 serveur MCP configuré"},
	"note.mcp_count": {EN: "%d MCP servers configured: %s", FR: "%d serveurs MCP configurés : %s"},
	"note.copilot_model_switch": {
		EN: "Copilot auto-routes models; a model switch empties the prompt cache (see the breakdown below).",
		FR: "Copilot route automatiquement les modèles ; changer de modèle vide le cache de prompt (voir le détail ci-dessous).",
	},

	// `ats fix` (CLI and TUI, shared where the wording matches).
	"fix.no_config":        {EN: "hermes: no config.yaml found, nothing to fix", FR: "hermes : aucun config.yaml trouvé, rien à corriger"},
	"fix.dry_run":          {EN: "dry run: re-run with --write to apply, or confirm interactively from a terminal", FR: "simulation : relancer avec --write pour appliquer, ou confirmer de manière interactive depuis un terminal"},
	"fix.nothing_to_fix":   {EN: "nothing to fix: every known setting already matches the recommendation", FR: "rien à corriger : tous les réglages connus correspondent déjà à la recommandation"},
	"fix.found_header_cli": {EN: "found 1 change:\n  [%s] %s\n  %s: %s -> %s\n  file: %s", FR: "1 changement trouvé :\n  [%s] %s\n  %s : %s -> %s\n  fichier : %s"},
	"fix.found_header_tui": {EN: "found 1 change:\n  [%s] %s\n\n  - %s: %s\n  + %s: %s\n\n  file: %s\n\npress enter to apply (confirm defaults to No)", FR: "1 changement trouvé :\n  [%s] %s\n\n  - %s : %s\n  + %s : %s\n\n  fichier : %s\n\nappuyer sur entrée pour appliquer (la confirmation défaut est Non)"},
	"fix.not_applied":      {EN: "not applied", FR: "non appliqué"},
	"fix.applied":          {EN: "applied: %s %s -> %s (backup: %s)", FR: "appliqué : %s %s -> %s (sauvegarde : %s)"},
	"fix.was_not_applied":  {EN: "fix was not applied", FR: "le correctif n'a pas été appliqué"},
	"fix.error":            {EN: "error: %v", FR: "erreur : %v"},

	"confirm.title":       {EN: "Apply %s: %s -> %s?", FR: "Appliquer %s : %s -> %s ?"},
	"confirm.affirmative": {EN: "Apply", FR: "Appliquer"},
	"confirm.negative":    {EN: "Skip", FR: "Ignorer"},

	// Errors/usage.
	"err.unknown_command":     {EN: "ats: unknown command %q", FR: "ats : commande inconnue %q"},
	"err.unexpected_argument": {EN: "ats: unexpected argument %q", FR: "ats : argument inattendu %q"},

	// usage.text is the full `ats --help` / `-h` / `help` / `<cmd>
	// --help` output (card: same usage text for the top-level and every
	// subcommand's --help, not a different page per command).
	"usage.text": {
		EN: "ats - where an AI coding agent's prompt cache wins or loses money, and which setting to change\n\n" +
			"Usage:\n" +
			"  ats [command] [flags]\n\n" +
			"Commands:\n" +
			"  cache       show cache hit rate and cost for each agent (default)\n" +
			"  dashboard   show the CodeBurn-style dashboard\n" +
			"  prefix      show what gets sent as the prompt prefix\n" +
			"  ttl         estimate the savings of a different cache TTL\n" +
			"  fix         apply a recommended configuration fix\n" +
			"  version     print the ats version\n\n" +
			"Flags:\n" +
			"  --days N      size of the look-back window in days (default 7)\n" +
			"  --json        print machine-readable JSON instead of styled text\n" +
			"  --no-tui      force the non-interactive text output\n" +
			"  --lang en|fr  select the UI language\n\n" +
			"Environment:\n" +
			"  ATS_LANG      select the UI language (en or fr)\n" +
			"  NO_COLOR      disable colored output\n",
		FR: "ats - ou le cache de prompt d'un agent de code IA gagne ou perd de l'argent, et quel réglage changer\n\n" +
			"Usage :\n" +
			"  ats [commande] [options]\n\n" +
			"Commandes :\n" +
			"  cache       taux de cache et coût par agent (par défaut)\n" +
			"  dashboard   tableau de bord façon CodeBurn\n" +
			"  prefix      ce qui est envoyé comme préfixe de prompt\n" +
			"  ttl         estimation de l'économie d'un autre TTL de cache\n" +
			"  fix         appliquer une correction de configuration recommandée\n" +
			"  version     afficher la version d'ats\n\n" +
			"Options :\n" +
			"  --days N      taille de la fenêtre d'historique en jours (défaut 7)\n" +
			"  --json        afficher du JSON exploitable au lieu du texte stylé\n" +
			"  --no-tui      forcer la sortie texte non interactive\n" +
			"  --lang en|fr  choisir la langue de l'interface\n\n" +
			"Environnement :\n" +
			"  ATS_LANG      choisir la langue de l'interface (en ou fr)\n" +
			"  NO_COLOR      désactiver les couleurs\n",
	},

	// Dashboard tab (tui.go's dashboardBody / dashboard.go /
	// dashboard_render.go): the card-mandated "numbers first, words
	// second" replacement for the old prose Resume tab.
	"dashboard.hero_cost":     {EN: "cost", FR: "coût"},
	"dashboard.hero_saved":    {EN: "saved via cache", FR: "économisé grâce au cache"},
	"dashboard.hero_hit_rate": {EN: "cache hit rate", FR: "taux de cache"},
	"dashboard.hero_tokens_line": {
		EN: "%s tok read from cache / %s tok cached / %s tok sent uncached / %s tok generated",
		FR: "%s tok lus du cache / %s tok mis en cache / %s tok envoyés sans cache / %s tok générés",
	},
	"dashboard.compare_up":   {EN: "Up %s points vs the previous period.", FR: "En hausse de %s points vs la période précédente."},
	"dashboard.compare_down": {EN: "Down %s points vs the previous period.", FR: "En baisse de %s points vs la période précédente."},

	// Hero mini-stats (card 1a): calls/sessions, then
	// yesterday/7-day-average/vs-yesterday swing, shown under the
	// hero's big figures.
	"dashboard.mini_calls_sessions": {EN: "%s calls \u00b7 %s sessions", FR: "%s appels \u00b7 %s sessions"},
	"dashboard.mini_yesterday":      {EN: "Yesterday %s", FR: "Hier %s"},
	"dashboard.mini_avg7":           {EN: "7-day avg %s", FR: "Moy. 7 jours %s"},
	"dashboard.mini_vs_yesterday":   {EN: "vs yesterday %s", FR: "vs hier %s"},

	"dashboard.panel_by_agent": {EN: "Cache rate by agent", FR: "Taux de cache par agent"},
	"dashboard.panel_by_day":   {EN: "By day", FR: "Par jour"},
	"dashboard.panel_by_model": {EN: "By model", FR: "Par modèle"},
	"dashboard.panel_improve":  {EN: "To improve", FR: "À améliorer"},
	"dashboard.panel_measured": {EN: "Measured", FR: "Mesuré"},

	"dashboard.panel_activity":      {EN: "Activity", FR: "Activité"},
	"dashboard.heatmap_active_days": {EN: "%d active days", FR: "%d jours actifs"},
	"dashboard.panel_histogram":     {EN: "Daily spend", FR: "Dépenses quotidiennes"},
	"dashboard.histogram_avg":       {EN: "Avg/day", FR: "Moy./jour"},
	"dashboard.histogram_peak":      {EN: "Peak", FR: "Pic"},
	"dashboard.histogram_yesterday": {EN: "Yesterday", FR: "Hier"},

	"dashboard.panel_efficiency": {EN: "Efficiency", FR: "Efficacité"},
	"dashboard.eff_score_line":   {EN: "%d/100  %s", FR: "%d/100  %s"},
	"dashboard.eff_cache_rate":   {EN: "Cache hit rate", FR: "Taux de cache"},
	"dashboard.eff_no_rebuild":   {EN: "No abnormal rebuild", FR: "Sans reconstruction anormale"},
	"dashboard.eff_ttl_tuned":    {EN: "TTL well tuned", FR: "TTL bien r\u00e9gl\u00e9"},
	"dashboard.yes":              {EN: "yes", FR: "oui"},
	"dashboard.no":               {EN: "no", FR: "non"},
	"dashboard.efficiency_help": {
		EN: "Efficiency score = 60% cache hit rate + 25% share of calls without an abnormal rebuild + 15% TTL well tuned (100 if no pending TTL fix, else 0), rounded, graded A+ to F.",
		FR: "Score d'efficacit\u00e9 = 60 % taux de cache + 25 % part d'appels sans reconstruction anormale + 15 % TTL bien r\u00e9gl\u00e9 (100 si aucun correctif TTL en attente, sinon 0), arrondi, not\u00e9 de A+ \u00e0 F.",
	},

	"dashboard.unknown_price_mark":     {EN: "-", FR: "-"},
	"dashboard.unknown_price_footnote": {EN: "unknown price for some agents", FR: "prix inconnu pour certains agents"},
	"dashboard.no_model_data":          {EN: "No per-model detail available.", FR: "Aucun détail par modèle disponible."},
	"dashboard.no_daily_data":          {EN: "No daily detail for this selection.", FR: "Aucun détail quotidien pour cette sélection."},
	"dashboard.no_agents":              {EN: "No agent data yet.", FR: "Aucune donnée d'agent pour l'instant."},
	"dashboard.nothing_to_fix":         {EN: "Nothing to fix.", FR: "Rien à corriger."},
	"dashboard.ignored_note":           {EN: "no data: %s", FR: "aucune donnée : %s"},

	"dashboard.finding_fix_hermes": {
		EN: "Hermes cache TTL misconfigured",
		FR: "TTL de cache Hermes mal réglé",
	},
	"dashboard.finding_ttl": {
		EN: "Cache TTL misconfigured (%s)",
		FR: "TTL de cache mal réglé (%s)",
	},
	"dashboard.finding_warm_rebuilds": {
		EN: "Abnormal rebuilds (%s)",
		FR: "Reconstructions anormales (%s)",
	},
	"dashboard.finding_copilot_switch": {
		EN: "Model switch empties Copilot cache",
		FR: "Changement modèle vide cache Copilot",
	},

	"dashboard.measured_sessions":          {EN: "%s sessions", FR: "%s sessions"},
	"dashboard.measured_sessions_no_usage": {EN: "%s without usage", FR: "%s sans usage"},

	// Panel B ("Taux de cache par agent") column headers.
	"dashboard.col_agent": {EN: "agent", FR: "agent"},
	"dashboard.col_cache": {EN: "cache", FR: "cache"},
	"dashboard.col_calls": {EN: "calls", FR: "appels"},
	"dashboard.col_cost":  {EN: "cost", FR: "co\u00fbt"},
	"dashboard.col_saved": {EN: "saved", FR: "\u00e9conomis\u00e9"},
	"dashboard.not_measured_cache_write": {
		EN: "cache writes not measured by %s",
		FR: "écritures de cache non mesurées par %s",
	},
	"dashboard.not_measured_price": {EN: "unknown price for this model", FR: "prix inconnu pour ce modèle"},
}

// T renders the catalog entry key in lang, Sprintf-formatted with args
// when any are given. A missing lang entry falls back to EN (should
// never happen once TestCatalogParity passes); a missing key renders the
// bare key so a typo is visible instead of silently disappearing.
func T(lang Lang, key string, args ...any) string {
	entry, ok := catalog[key]
	if !ok {
		return key
	}
	tmpl, ok := entry[lang]
	if !ok {
		tmpl = entry[EN]
	}
	if len(args) == 0 {
		return tmpl
	}
	return fmt.Sprintf(tmpl, args...)
}
