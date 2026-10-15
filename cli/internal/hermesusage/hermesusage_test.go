package hermesusage

import (
	"testing"
)

func TestSummarizePerCallAvoidsDivisionByZero(t *testing.T) {
	if got := PerCall(5, 0); got != 0 {
		t.Fatalf("PerCall(5, 0) = %d, want 0", got)
	}
	if got := PerCall(100, 10); got != 10 {
		t.Fatalf("PerCall(100, 10) = %d, want 10", got)
	}
}
