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
		"clem quality run --home",
		"Quality gates finished",
		"Quality gates exhausted max attempts",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("runner missing %q", want)
		}
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
