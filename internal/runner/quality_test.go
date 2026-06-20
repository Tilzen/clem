package runner

import (
	"strings"
	"testing"

	"github.com/jahwag/clem/internal/config"
)

func TestGenerate_QualityGatesEnabled(t *testing.T) {
	cfg := baseCfg("lead", config.AgentConfig{
		Name:      "Lead",
		Model:     "claude-sonnet-4-6",
		Iteration: "1m",
		Prompt:    "work",
	})
	cfg.Quality = &config.QualityConfig{
		Enabled:   true,
		OnFailure: "feedback",
		Gates: []config.QualityGate{
			{Name: "unit", Level: "unit", Cmd: "go test ./...", Blocking: true},
		},
	}
	out := Generate(cfg, "lead")
	for _, want := range []string{
		"quality-feedback.txt",
		// clem must be referenced by absolute path (agent PATH is not trusted).
		"/usr/local/bin/clem quality run --home",
		"Quality gates finished",
		// rc 127 (binary missing) is logged distinctly, not treated as a pass.
		`QUALITY_RC" -eq 127`,
		"quality gates skipped",
		// rc 2 = transition into blocked → alert once.
		`QUALITY_RC" -eq 2`,
		"Quality gates exhausted max attempts",
		// rc 3 = already-blocked no-op → log only, no alert.
		`QUALITY_RC" -eq 3`,
		"no-op (alert already sent)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("runner missing %q", want)
		}
	}
	// The bare (PATH-dependent) invocation must be gone.
	if strings.Contains(out, "\n        clem quality run") {
		t.Error("runner must not invoke clem via bare PATH")
	}
}

// TestGenerate_QualityAlertsOnlyOnTransition guards FIX 2: the runner fires the
// [BLOCKED] alert only on rc 2 (the transition), never on rc 3 (already
// blocked). The alert branch must sit between the rc==2 and rc==3 branches.
func TestGenerate_QualityAlertsOnlyOnTransition(t *testing.T) {
	cfg := baseCfg("lead", config.AgentConfig{
		Name: "Lead", Model: "claude-sonnet-4-6", Iteration: "1m", Prompt: "work",
	})
	cfg.Quality = &config.QualityConfig{
		Enabled:   true,
		OnFailure: "feedback",
		Gates:     []config.QualityGate{{Name: "unit", Level: "unit", Cmd: "go test ./...", Blocking: true}},
	}
	out := Generate(cfg, "lead")

	idx2 := strings.Index(out, `QUALITY_RC" -eq 2`)
	idx3 := strings.Index(out, `QUALITY_RC" -eq 3`)
	idxAlert := strings.Index(out, "Quality gates exhausted max attempts")
	if idx2 < 0 || idx3 < 0 || idxAlert < 0 {
		t.Fatalf("expected rc 2, rc 3 and alert branches in:\n%s", out)
	}
	// The alert log line must fall inside the rc==2 branch: after rc==2 and
	// before rc==3 (the no-op branch carries no alert).
	if !(idx2 < idxAlert && idxAlert < idx3) {
		t.Errorf("alert must live in the rc==2 branch only (idx2=%d alert=%d idx3=%d)", idx2, idxAlert, idx3)
	}
}

func TestGenerate_QualityDisabledOmitsHooks(t *testing.T) {
	cfg := baseCfg("lead", config.AgentConfig{
		Name: "Lead", Model: "claude-sonnet-4-6", Iteration: "1m", Prompt: "work",
	})
	out := Generate(cfg, "lead")
	if strings.Contains(out, "clem quality run") {
		t.Error("quality run should be absent when disabled")
	}
}

func TestGenerate_QualityFeedbackPrependedBeforeSession(t *testing.T) {
	cfg := baseCfg("lead", config.AgentConfig{
		Name: "Lead", Model: "claude-sonnet-4-6", Iteration: "1m", Prompt: "work",
	})
	cfg.Quality = &config.QualityConfig{
		Enabled: true,
		Gates:   []config.QualityGate{{Name: "unit", Level: "unit", Cmd: "true", Blocking: true}},
	}
	out := Generate(cfg, "lead")
	if !strings.Contains(out, `quality-feedback.txt`) || !strings.Contains(out, "RUNNER_WARNINGS") {
		t.Fatalf("runner should prepend quality feedback before session:\n%s", out)
	}
}
