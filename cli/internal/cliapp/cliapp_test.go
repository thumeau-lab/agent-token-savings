package cliapp

import (
	"bytes"
	"strings"
	"testing"
)

func TestVersionPrintsSemverLikeString(t *testing.T) {
	var out, errOut bytes.Buffer
	code := Run([]string{"version"}, &out, &errOut, Env{})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, errOut.String())
	}
	if !strings.HasPrefix(out.String(), "ats ") {
		t.Fatalf("output = %q, want prefix %q", out.String(), "ats ")
	}
}
