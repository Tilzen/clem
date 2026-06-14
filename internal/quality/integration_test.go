package quality_test

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jahwag/clem/internal/config"
	"github.com/jahwag/clem/internal/quality"
)

// harness simulates an agent home + project workdir for integration tests.
type harness struct {
	Home     string
	Work     string
	AgentKey string
	RC       quality.RuntimeConfig
}

func newHarness(t *testing.T, gateCmd string, maxAttempts int) *harness {
	t.Helper()
	home := t.TempDir()
	work := filepath.Join(home, "demo")
	if err := os.MkdirAll(work, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(work, "CLAUDE.local.md"), []byte("# agent context\n"), 0644); err != nil {
		t.Fatal(err)
	}
	rc := quality.RuntimeConfig{
		Enabled:     true,
		OnFailure:   "feedback",
		MaxAttempts: maxAttempts,
		AgentKey:    "lead",
		Project:     "demo",
		Gates: []quality.Gate{
			{Name: "check", Level: "unit", Cmd: gateCmd, Blocking: true},
		},
	}
	if err := quality.WriteRuntimeConfig(home, rc); err != nil {
		t.Fatal(err)
	}
	return &harness{Home: home, Work: work, AgentKey: "lead", RC: rc}
}

func (h *harness) runIteration(t *testing.T) (int, error) {
	t.Helper()
	return quality.RunIteration(h.Home, h.Work, h.RC)
}

func (h *harness) markerPath() string {
	return filepath.Join(h.Work, ".quality_ok")
}

func (h *harness) setOK(t *testing.T) {
	t.Helper()
	if err := os.WriteFile(h.markerPath(), []byte("1"), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestIntegration_ClosedLoopConvergence(t *testing.T) {
	h := newHarness(t, "test -f .quality_ok", 5)

	code, err := h.runIteration(t)
	if code != 1 || err == nil {
		t.Fatalf("iter1: want code 1 + err, got %d %v", code, err)
	}
	fb, _ := os.ReadFile(quality.FeedbackPath(h.Home))
	if len(fb) == 0 {
		t.Fatal("iter1: expected feedback file")
	}
	claude, _ := os.ReadFile(filepath.Join(h.Work, "CLAUDE.local.md"))
	if !strings.Contains(string(claude), quality.FeedbackStartMarker) {
		t.Fatal("iter1: CLAUDE.local.md should contain feedback block")
	}
	state, _ := quality.LoadState(h.Home)
	if state.Attempts != 1 {
		t.Fatalf("iter1: attempts = %d, want 1", state.Attempts)
	}

	h.setOK(t)
	code, err = h.runIteration(t)
	if code != 0 || err != nil {
		t.Fatalf("iter2: want pass, got %d %v", code, err)
	}
	if _, err := os.Stat(quality.FeedbackPath(h.Home)); !os.IsNotExist(err) {
		t.Fatal("iter2: feedback file should be cleared on pass")
	}
	claude, _ = os.ReadFile(filepath.Join(h.Work, "CLAUDE.local.md"))
	if strings.Contains(string(claude), quality.FeedbackStartMarker) {
		t.Fatal("iter2: feedback block should be cleared from CLAUDE.local.md")
	}
	state, _ = quality.LoadState(h.Home)
	if state.Attempts != 0 {
		t.Fatalf("iter2: attempts reset, got %d", state.Attempts)
	}

	entries := h.readJSONL(t)
	if len(entries) < 2 {
		t.Fatalf("expected >=2 jsonl entries, got %d", len(entries))
	}
	if !entries[len(entries)-1].Pass {
		t.Fatal("last jsonl entry should be pass=true")
	}
}

func TestIntegration_MaxAttemptsBlocks(t *testing.T) {
	h := newHarness(t, "exit 1", 2)

	code, _ := h.runIteration(t)
	if code != 1 {
		t.Fatalf("first fail: code=%d", code)
	}
	code, err := h.runIteration(t)
	if code != 2 {
		t.Fatalf("second fail: want code 2 (blocked), got %d err=%v", code, err)
	}
	state, _ := quality.LoadState(h.Home)
	if !state.Blocked {
		t.Fatal("state should be blocked")
	}
	code, _ = h.runIteration(t)
	if code != 2 {
		t.Fatalf("third run on blocked task: want code 2, got %d", code)
	}
}

func TestIntegration_TaskChangeResetsAttempts(t *testing.T) {
	h := newHarness(t, "exit 1", 5)
	if _, err := h.runIteration(t); err == nil {
		t.Fatal("expected fail")
	}
	state, _ := quality.LoadState(h.Home)
	if state.Attempts != 1 {
		t.Fatalf("attempts=%d", state.Attempts)
	}

	if err := os.WriteFile(quality.TaskIDPath(h.Home), []byte("task-b\n"), 0600); err != nil {
		t.Fatal(err)
	}
	state, err := quality.ResetAttemptsIfTaskChanged(h.Home, "task-b")
	if err != nil || state.Attempts != 0 {
		t.Fatalf("reset: state=%+v err=%v", state, err)
	}
	code, _ := h.runIteration(t)
	if code != 1 {
		t.Fatalf("new task first fail: code=%d", code)
	}
	state, _ = quality.LoadState(h.Home)
	if state.TaskID != "task-b" || state.Attempts != 1 {
		t.Fatalf("state after new task: %+v", state)
	}
}

func TestIntegration_BDDGateFeedback(t *testing.T) {
	home := t.TempDir()
	work := filepath.Join(home, "demo")
	if err := os.MkdirAll(work, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(work, "CLAUDE.local.md"), []byte("# ctx\n"), 0644); err != nil {
		t.Fatal(err)
	}
	rc := quality.RuntimeConfig{
		Enabled:     true,
		OnFailure:   "feedback",
		MaxAttempts: 3,
		AgentKey:    "lead",
		Gates: []quality.Gate{
			{
				Name:     "acceptance",
				Kind:     quality.KindBDD,
				Level:    "acceptance",
				Specs:    []string{"features/"},
				Cmd:      `echo "Feature: Greeting\nScenario: hello\nexpected hello got bye"; exit 1`,
				Blocking: true,
			},
		},
	}
	code, err := quality.RunIteration(home, work, rc)
	if code != 1 || err == nil {
		t.Fatalf("code=%d err=%v", code, err)
	}
	fb, _ := os.ReadFile(quality.FeedbackPath(home))
	body := string(fb)
	for _, want := range []string{"Living specs: features/", "Feature: Greeting", "Scenario:"} {
		if !strings.Contains(body, want) {
			t.Errorf("feedback missing %q:\n%s", want, body)
		}
	}
}

func TestIntegration_AdvisoryNeverBlocksIteration(t *testing.T) {
	h := newHarness(t, "exit 1", 3)
	h.RC.OnFailure = "advisory"
	code, err := h.runIteration(t)
	if code != 0 || err != nil {
		t.Fatalf("advisory should return 0, got %d %v", code, err)
	}
	if _, err := os.Stat(quality.FeedbackPath(h.Home)); err == nil {
		t.Fatal("advisory should not write feedback file")
	}
}

func TestIntegration_ConfigRoundTrip(t *testing.T) {
	path := writeConfigYAML(t, `
project: demo
coordination:
  backend: discord
  server_id: "1"
  channels: {general: "1", alerts: "2"}
quality:
  enabled: true
  max_attempts: 4
  gates:
    - name: unit
      level: unit
      cmd: "test -f .quality_ok"
      blocking: true
    - name: bdd
      kind: bdd
      level: acceptance
      specs: [features/]
      runner: godog
      blocking: true
agents:
  lead:
    name: Lead
    model: claude-sonnet-4-6
    quality_suite: [unit, bdd]
`)
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	rc, err := cfg.RuntimeQualityForAgent("lead")
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	if err := quality.WriteRuntimeConfig(home, rc); err != nil {
		t.Fatal(err)
	}
	loaded, err := quality.LoadRuntimeConfig(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Gates) != 2 || loaded.Gates[1].Cmd != "godog features/" {
		t.Fatalf("loaded gates: %+v", loaded.Gates)
	}
}

func TestIntegration_BaselineAndAgentRestricted(t *testing.T) {
	path := writeConfigYAML(t, `
project: demo
coordination:
  backend: discord
  server_id: "1"
  channels: {general: "1", alerts: "2"}
quality:
  enabled: true
  baseline_suite: [unit]
  gates:
    - name: unit
      level: unit
      cmd: "true"
      blocking: true
    - name: acceptance
      kind: bdd
      level: acceptance
      specs: [features/]
      runner: godog
      agents: [lead]
      blocking: true
agents:
  lead:
    name: Lead
    model: claude-sonnet-4-6
    quality_suite: [+acceptance]
  reviewer:
    name: Reviewer
    model: claude-sonnet-4-6
`)
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	leadRC, err := cfg.RuntimeQualityForAgent("lead")
	if err != nil {
		t.Fatal(err)
	}
	if len(leadRC.Gates) != 2 || leadRC.Gates[1].Name != "acceptance" {
		t.Fatalf("lead gates: %+v", leadRC.Gates)
	}
	reviewerRC, err := cfg.RuntimeQualityForAgent("reviewer")
	if err != nil {
		t.Fatal(err)
	}
	if len(reviewerRC.Gates) != 1 || reviewerRC.Gates[0].Name != "unit" {
		t.Fatalf("reviewer gates: %+v", reviewerRC.Gates)
	}
}

func TestRunSuite_GateTimeout(t *testing.T) {
	dir := t.TempDir()
	gates := []quality.Gate{
		{Name: "slow", Level: "custom", Cmd: "sleep 2", Blocking: true, Timeout: 100 * time.Millisecond},
	}
	v := quality.RunSuite(dir, gates, "t", 1)
	if v.Pass {
		t.Fatal("expected timeout failure")
	}
	if len(v.Results) != 1 || v.Results[0].ExitCode != 124 {
		t.Fatalf("results=%+v", v.Results)
	}
}

func TestRunSuite_GateRetries(t *testing.T) {
	dir := t.TempDir()
	// Fails first time, succeeds second — simulate with a counter file
	script := `n=$(cat .retry_count 2>/dev/null || echo 0); n=$((n+1)); echo $n > .retry_count; [ "$n" -ge 2 ]`
	gates := []quality.Gate{
		{Name: "flaky", Level: "unit", Cmd: script, Blocking: true, Retries: 2},
	}
	v := quality.RunSuite(dir, gates, "t", 1)
	if !v.Pass {
		t.Fatalf("expected pass after retry, results=%+v", v.Results)
	}
}

func (h *harness) readJSONL(t *testing.T) []quality.JSONLEntry {
	t.Helper()
	f, err := os.Open(quality.JSONLPath(h.Home, h.AgentKey))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var out []quality.JSONLEntry
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var e quality.JSONLEntry
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			t.Fatal(err)
		}
		out = append(out, e)
	}
	return out
}

func writeConfigYAML(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "clem.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}
