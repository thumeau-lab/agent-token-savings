package ttlestimate

import "testing"

func TestEstimateCountsOnlyMachineSessionsWithWrites(t *testing.T) {
	db := buildFixtureHome(t, []fixtureSession{
		{id: "human1", source: "cli", writeTokens: 999_000, gapSeconds: 1000},    // not a machine source: ignored for sess/gap, counted in w_all
		{id: "m-nogap", source: "cron", writeTokens: 100_000, gapSeconds: 60},    // machine, gap<=5min
		{id: "m-gap", source: "subagent", writeTokens: 200_000, gapSeconds: 600}, // machine, gap>5min
	})

	res, err := Estimate(db, 30)
	if err != nil {
		t.Fatalf("Estimate: %v", err)
	}
	if res.MachineSessions != 2 {
		t.Fatalf("MachineSessions = %d, want 2", res.MachineSessions)
	}
	if res.GapSessions != 1 {
		t.Fatalf("GapSessions = %d, want 1", res.GapSessions)
	}
	if res.MachineWriteTokens != 300_000 {
		t.Fatalf("MachineWriteTokens = %d, want 300000", res.MachineWriteTokens)
	}
}
