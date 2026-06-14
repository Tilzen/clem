package config

import (
	"strings"
	"testing"

	"github.com/jahwag/clem/internal/quality"
)

func TestLoad_QualityConfigValid(t *testing.T) {
	path := writeYAML(t, `
project: myteam
coordination:
  backend: discord
  server_id: "1"
  channels: {general: "g", alerts: "a"}
quality:
  enabled: true
  on_failure: feedback
  max_attempts: 2
  gates:
    - name: build
      level: build
      cmd: "go build ./..."
      blocking: true
      timeout: 60s
agents:
  lead:
    name: "Lead"
    model: "claude-sonnet-4-6"
    quality_suite: [build]
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	gates, err := cfg.SuiteForAgent("lead")
	if err != nil || len(gates) != 1 || gates[0].Name != "build" {
		t.Fatalf("suite = %+v err=%v", gates, err)
	}
}

func TestLoad_QualityUnknownGateRejected(t *testing.T) {
	path := writeYAML(t, `
project: myteam
coordination:
  backend: discord
  server_id: "1"
  channels: {general: "g"}
quality:
  enabled: true
  gates:
    - name: unit
      level: unit
      cmd: "true"
agents:
  lead:
    name: "Lead"
    quality_suite: [missing]
`)
	if _, err := Load(path); err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatalf("expected unknown gate error, got %v", err)
	}
}

func TestLoad_QualityDisabledByDefault(t *testing.T) {
	path := writeYAML(t, `
project: myteam
coordination:
  backend: discord
  server_id: "1"
  channels: {general: "g"}
agents:
  lead:
    name: "Lead"
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Quality != nil && cfg.Quality.Enabled {
		t.Fatal("quality should be disabled by default")
	}
}

func TestQualityGate_ResolvedCmdBehaveCucumber(t *testing.T) {
	for _, tc := range []struct {
		runner, want string
	}{
		{"behave", "behave features/"},
		{"cucumber", "npx cucumber-js features/"},
	} {
		g := QualityGate{Kind: "bdd", Specs: []string{"features/"}, Runner: tc.runner}
		cmd, err := g.ResolvedCmd()
		if err != nil || cmd != tc.want {
			t.Fatalf("%s: cmd=%q err=%v", tc.runner, cmd, err)
		}
	}
}

func TestLoad_QualityBDDGateResolvesGodog(t *testing.T) {
	path := writeYAML(t, `
project: myteam
coordination:
  backend: discord
  server_id: "1"
  channels: {general: "g"}
quality:
  enabled: true
  gates:
    - name: acceptance
      kind: bdd
      level: acceptance
      specs: [features/]
      runner: godog
      blocking: true
agents:
  lead:
    name: "Lead"
    model: "claude-sonnet-4-6"
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	gates, err := cfg.SuiteForAgent("lead")
	if err != nil {
		t.Fatal(err)
	}
	if len(gates) != 1 || gates[0].Cmd != "godog features/" {
		t.Fatalf("got %+v", gates[0])
	}
	if gates[0].Kind != "bdd" {
		t.Fatalf("kind = %q", gates[0].Kind)
	}
}

func TestSuiteForAgent_BaselineAndAgentRestricted(t *testing.T) {
	path := writeYAML(t, `
project: myteam
coordination:
  backend: discord
  server_id: "1"
  channels: {general: "g"}
quality:
  enabled: true
  baseline_suite: [build, unit]
  gates:
    - name: build
      level: build
      cmd: "go build ./..."
      blocking: true
    - name: unit
      level: unit
      cmd: "go test ./..."
      blocking: true
    - name: lint
      level: lint
      cmd: "golangci-lint run"
      blocking: false
    - name: acceptance
      level: acceptance
      kind: bdd
      specs: [features/]
      runner: godog
      agents: [lead]
      blocking: true
agents:
  lead:
    name: "Lead"
  reviewer:
    name: "Reviewer"
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	lead, err := cfg.SuiteForAgent("lead")
	if err != nil {
		t.Fatal(err)
	}
	if len(lead) != 3 {
		t.Fatalf("lead suite = %d gates, want 3: %+v", len(lead), gateNames(lead))
	}
	if lead[0].Name != "build" || lead[1].Name != "unit" || lead[2].Name != "acceptance" {
		t.Fatalf("lead order = %v", gateNames(lead))
	}
	reviewer, err := cfg.SuiteForAgent("reviewer")
	if err != nil {
		t.Fatal(err)
	}
	if len(reviewer) != 2 || gateNames(reviewer)[0] != "build" {
		t.Fatalf("reviewer suite = %+v", gateNames(reviewer))
	}
}

func TestSuiteForAgent_InheritAddsExtra(t *testing.T) {
	path := writeYAML(t, `
project: myteam
coordination:
  backend: discord
  server_id: "1"
  channels: {general: "g"}
quality:
  enabled: true
  baseline_suite: [build, unit]
  gates:
    - name: build
      level: build
      cmd: "go build ./..."
      blocking: true
    - name: unit
      level: unit
      cmd: "go test ./..."
      blocking: true
    - name: lint
      level: lint
      cmd: "golangci-lint run"
      blocking: false
agents:
  lead:
    name: "Lead"
    quality_suite: [+lint]
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	gates, err := cfg.SuiteForAgent("lead")
	if err != nil {
		t.Fatal(err)
	}
	if len(gates) != 3 || gates[2].Name != "lint" {
		t.Fatalf("got %+v", gateNames(gates))
	}
}

func TestSuiteForAgent_InlineQualityGates(t *testing.T) {
	path := writeYAML(t, `
project: myteam
coordination:
  backend: discord
  server_id: "1"
  channels: {general: "g"}
quality:
  enabled: true
  baseline_suite: [unit]
  gates:
    - name: unit
      level: unit
      cmd: "go test ./..."
      blocking: true
agents:
  lead:
    name: "Lead"
    quality_gates:
      - name: lead-smoke
        level: custom
        cmd: "./scripts/lead-smoke.sh"
        blocking: true
    quality_suite: [inherit, +lead-smoke]
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	gates, err := cfg.SuiteForAgent("lead")
	if err != nil {
		t.Fatal(err)
	}
	if len(gates) != 2 || gates[1].Name != "lead-smoke" {
		t.Fatalf("got %+v", gateNames(gates))
	}
}

func TestLoad_AgentRestrictedGateNotInExplicitSuiteForOtherAgent(t *testing.T) {
	path := writeYAML(t, `
project: myteam
coordination:
  backend: discord
  server_id: "1"
  channels: {general: "g"}
quality:
  enabled: true
  gates:
    - name: unit
      level: unit
      cmd: "true"
      blocking: true
    - name: lead-only
      level: custom
      cmd: "true"
      agents: [lead]
      blocking: true
agents:
  reviewer:
    name: "Reviewer"
    quality_suite: [unit, lead-only]
`)
	if _, err := Load(path); err == nil || !strings.Contains(err.Error(), "lead-only") {
		t.Fatalf("expected disallowed gate error, got %v", err)
	}
}

func TestLoad_UnknownAgentInGateAgentsRejected(t *testing.T) {
	path := writeYAML(t, `
project: myteam
coordination:
  backend: discord
  server_id: "1"
  channels: {general: "g"}
quality:
  enabled: true
  gates:
    - name: unit
      level: unit
      cmd: "true"
      agents: [missing]
      blocking: true
agents:
  lead:
    name: "Lead"
`)
	if _, err := Load(path); err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatalf("expected unknown agent error, got %v", err)
	}
}

func gateNames(gates []quality.Gate) []string {
	names := make([]string, len(gates))
	for i, g := range gates {
		names[i] = g.Name
	}
	return names
}
