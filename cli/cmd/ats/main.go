// Command ats is cli/main.go's entry point rehomed under a directory
// whose last path element is "ats", so
// `go install github.com/thumeau-lab/agent-token-savings/cli/cmd/ats@latest`
// installs a binary literally named "ats" (go install derives the
// binary name from the import path's last element -- for the module
// root that is "cli", see cli/main.go). Both this file and
// cli/main.go call the exact same atsmain.Run, so there is only one
// real implementation to keep in sync.
package main

import (
	"os"

	"github.com/thumeau-lab/agent-token-savings/cli/internal/atsmain"
)

func main() {
	os.Exit(atsmain.Run())
}
