// Command ats explains where an AI coding agent's prompt cache wins or loses
// money, and which setting to change. Offline, no telemetry, no network at
// runtime. See README.md for the full command reference.
//
// `go install github.com/thumeau-lab/agent-token-savings/cli@latest`
// names the resulting binary "cli" (go install derives the binary
// name from the import path's last element, here the module root
// directory). See cli/cmd/ats/main.go for the path that installs a
// binary literally named "ats" instead.
package main

import (
	"os"

	"github.com/thumeau-lab/agent-token-savings/cli/internal/atsmain"
)

func main() {
	os.Exit(atsmain.Run())
}
