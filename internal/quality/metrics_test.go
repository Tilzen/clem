package quality_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jahwag/clem/internal/quality"
)

func TestReadAgentSummary_LastAttemptGroup(t *testing.T) {
	home := t.TempDir()
	agent := "lead"
	ts1 := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	ts2 := ts1.Add(time.Minute)

	v1 := quality.Verdict{
		Timestamp: ts1,
		TaskID:    "task-a",
		Attempt:   1,
		Results: []quality.GateResult{
			{Name: "unit", Pass: false, Blocking: true},
			{Name: "lint", Pass: true, Blocking: false},
		},
	}
	v2 := quality.Verdict{
		Timestamp: ts2,
		TaskID:    "task-a",
		Attempt:   2,
		Results: []quality.GateResult{
			{Name: "unit", Pass: true, Blocking: true},
			{Name: "lint", Pass: true, Blocking: false},
		},
	}
	if err := quality.AppendJSONL(home, agent, v1); err != nil {
		t.Fatal(err)
	}
	if err := quality.AppendJSONL(home, agent, v2); err != nil {
		t.Fatal(err)
	}

	sum, err := quality.ReadAgentSummary(home, agent)
	if err != nil {
		t.Fatal(err)
	}
	if sum.LastAttempt != 2 {
		t.Fatalf("LastAttempt=%d, want 2", sum.LastAttempt)
	}
	if !sum.LastPass {
		t.Fatal("expected LastPass=true for attempt 2")
	}
	if sum.GatesTotal != 2 || sum.GatesPass != 2 {
		t.Fatalf("gates pass/total = %d/%d, want 2/2", sum.GatesPass, sum.GatesTotal)
	}
	if sum.PassRate <= 0 {
		t.Fatalf("PassRate=%v, want > 0", sum.PassRate)
	}
}

func TestReadAgentSummary_MissingFile(t *testing.T) {
	sum, err := quality.ReadAgentSummary(t.TempDir(), "lead")
	if err != nil {
		t.Fatal(err)
	}
	if sum.AgentKey != "lead" || sum.GatesTotal != 0 {
		t.Fatalf("unexpected summary: %+v", sum)
	}
}

func TestAggregateGateStats(t *testing.T) {
	home := t.TempDir()
	agent := "lead"
	v := quality.Verdict{
		Timestamp: time.Now().UTC(),
		TaskID:    "t",
		Attempt:   1,
		Results: []quality.GateResult{
			{Name: "unit", Pass: true, DurationMS: 10},
			{Name: "unit", Pass: false, DurationMS: 30},
			{Name: "lint", Pass: true, DurationMS: 20},
		},
	}
	if err := quality.AppendJSONL(home, agent, v); err != nil {
		t.Fatal(err)
	}
	stats, err := quality.AggregateGateStats(home, agent)
	if err != nil {
		t.Fatal(err)
	}
	if len(stats) != 2 {
		t.Fatalf("expected 2 gate stats, got %d", len(stats))
	}
	byName := map[string]quality.GateStats{}
	for _, s := range stats {
		byName[s.Name] = s
	}
	unit := byName["unit"]
	if unit.Runs != 2 || unit.Passes != 1 {
		t.Fatalf("unit stats: %+v", unit)
	}
	if unit.PassRate != 0.5 {
		t.Fatalf("unit pass rate = %v, want 0.5", unit.PassRate)
	}
	if unit.AvgMS != 20 {
		t.Fatalf("unit avg ms = %d, want 20", unit.AvgMS)
	}
}

func TestWriteLoadRuntimeConfig(t *testing.T) {
	home := t.TempDir()
	rc := quality.RuntimeConfig{
		Enabled:     true,
		OnFailure:   "feedback",
		MaxAttempts: 5,
		AgentKey:    "lead",
		Project:     "demo",
		Gates: []quality.Gate{
			{Name: "unit", Level: "unit", Cmd: "true", Blocking: true},
		},
	}
	if err := quality.WriteRuntimeConfig(home, rc); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(home, ".clem", "quality.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	loaded, err := quality.LoadRuntimeConfig(home)
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.Enabled || loaded.MaxAttempts != 5 || len(loaded.Gates) != 1 {
		t.Fatalf("loaded: %+v", loaded)
	}
}
