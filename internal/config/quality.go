package config

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/jahwag/clem/internal/quality"
)

// QualityConfig defines deterministic quality gates for agent output.
type QualityConfig struct {
	Enabled       bool          `yaml:"enabled"`
	OnFailure     string        `yaml:"on_failure"`
	MaxAttempts   int           `yaml:"max_attempts"`
	BaselineSuite []string      `yaml:"baseline_suite"`
	Gates         []QualityGate `yaml:"gates"`
}

// QualityGate is one deterministic check in the quality suite.
//
// kind:
//   - command (default): run cmd verbatim — any deterministic check
//   - bdd: living Gherkin specs (features/) executed as acceptance gates
//
// For kind=bdd, set specs to feature paths and optionally runner (godog,
// behave, cucumber). cmd overrides the auto-resolved runner command.
type QualityGate struct {
	Name     string   `yaml:"name"`
	Kind     string   `yaml:"kind"`
	Level    string   `yaml:"level"`
	Cmd      string   `yaml:"cmd"`
	Specs    []string `yaml:"specs"`
	Runner   string   `yaml:"runner"`
	Blocking bool     `yaml:"blocking"`
	Timeout  string   `yaml:"timeout"`
	Retries  int      `yaml:"retries"`
	// Agents restricts this gate to specific agent keys. Empty = any agent.
	Agents []string `yaml:"agents"`
}

func (q *QualityConfig) validate() error {
	if q == nil || !q.Enabled {
		return nil
	}
	switch q.OnFailure {
	case "", "feedback", "block-push", "advisory":
	default:
		return fmt.Errorf("quality.on_failure must be feedback, block-push, or advisory, got %q", q.OnFailure)
	}
	if q.MaxAttempts < 0 {
		return fmt.Errorf("quality.max_attempts must be >= 0, got %d", q.MaxAttempts)
	}
	if len(q.Gates) == 0 {
		return fmt.Errorf("quality.enabled is true but no gates are defined")
	}
	names := make(map[string]bool, len(q.Gates))
	for i, g := range q.Gates {
		if g.Name == "" {
			return fmt.Errorf("quality.gates[%d]: name is required", i)
		}
		if names[g.Name] {
			return fmt.Errorf("quality.gates: duplicate name %q", g.Name)
		}
		names[g.Name] = true
		if !quality.ValidLevel(g.Level) {
			return fmt.Errorf("quality.gates[%d] (%s): unknown level %q (valid: %s)", i, g.Name, g.Level, strings.Join(quality.LevelOrder, ", "))
		}
		if !quality.ValidKind(g.Kind) {
			return fmt.Errorf("quality.gates[%d] (%s): unknown kind %q (valid: %s)", i, g.Name, g.Kind, strings.Join(quality.ValidKinds, ", "))
		}
		if _, err := g.ResolvedCmd(); err != nil {
			return fmt.Errorf("quality.gates[%d] (%s): %w", i, g.Name, err)
		}
		if g.Runner != "" && g.KindOrDefault() == quality.KindBDD {
			switch g.Runner {
			case "godog", "behave", "cucumber":
			default:
				return fmt.Errorf("quality.gates[%d] (%s): bdd runner must be godog, behave, or cucumber, got %q", i, g.Name, g.Runner)
			}
		}
		if g.Timeout != "" {
			if _, err := time.ParseDuration(g.Timeout); err != nil {
				return fmt.Errorf("quality.gates[%d] (%s): invalid timeout %q: %w", i, g.Name, g.Timeout, err)
			}
		}
		if g.Retries < 0 {
			return fmt.Errorf("quality.gates[%d] (%s): retries must be >= 0", i, g.Name)
		}
	}
	for _, name := range q.BaselineSuite {
		if !names[name] {
			return fmt.Errorf("quality.baseline_suite references unknown gate %q", name)
		}
	}
	return nil
}

func (q *QualityConfig) validateAgents(agentKeys map[string]bool) error {
	if q == nil || !q.Enabled {
		return nil
	}
	for i, g := range q.Gates {
		for _, key := range g.Agents {
			if !agentKeys[key] {
				return fmt.Errorf("quality.gates[%d] (%s): agents references unknown agent %q", i, g.Name, key)
			}
		}
	}
	return nil
}

// OnFailureOrDefault returns the configured on_failure mode.
func (q *QualityConfig) OnFailureOrDefault() string {
	if q == nil || q.OnFailure == "" {
		return "feedback"
	}
	return q.OnFailure
}

// MaxAttemptsOrDefault returns max closed-loop attempts.
func (q *QualityConfig) MaxAttemptsOrDefault() int {
	if q == nil || q.MaxAttempts == 0 {
		return 3
	}
	return q.MaxAttempts
}

// GateByName returns a gate definition by name.
func (q *QualityConfig) GateByName(name string) (QualityGate, bool) {
	if q == nil {
		return QualityGate{}, false
	}
	for _, g := range q.Gates {
		if g.Name == name {
			return g, true
		}
	}
	return QualityGate{}, false
}

// SuiteForAgent resolves the ordered gate list for an agent key.
//
// Default (empty quality_suite): baseline gates + agent-restricted global gates
// + inline quality_gates for that agent.
//
// Explicit quality_suite: exact gate list in order.
//
// Inherit mode (quality_suite contains "inherit" or "+name"): starts from the
// default suite, then appends extra gate names.
func (c *Config) SuiteForAgent(agentKey string) ([]quality.Gate, error) {
	if c.Quality == nil || !c.Quality.Enabled {
		return nil, nil
	}
	ac, ok := c.Agents[agentKey]
	if !ok {
		return nil, fmt.Errorf("unknown agent %q", agentKey)
	}
	if len(ac.QualitySuite) == 0 {
		return c.defaultSuiteForAgent(agentKey)
	}
	if suiteUsesInherit(ac.QualitySuite) {
		return c.inheritSuiteForAgent(agentKey, ac.QualitySuite)
	}
	var selected []QualityGate
	for _, name := range ac.QualitySuite {
		g, ok := c.gateForAgent(agentKey, name)
		if !ok {
			return nil, fmt.Errorf("agent %s: quality_suite references unknown or disallowed gate %q", agentKey, name)
		}
		selected = append(selected, g)
	}
	return toRuntimeGates(selected), nil
}

func gateAllowedForAgent(g QualityGate, agentKey string) bool {
	if len(g.Agents) == 0 {
		return true
	}
	return slices.Contains(g.Agents, agentKey)
}

func (c *Config) gateForAgent(agentKey, name string) (QualityGate, bool) {
	if c.Quality == nil {
		return QualityGate{}, false
	}
	if g, ok := c.Quality.GateByName(name); ok && gateAllowedForAgent(g, agentKey) {
		return g, true
	}
	ac, ok := c.Agents[agentKey]
	if !ok {
		return QualityGate{}, false
	}
	for _, g := range ac.QualityGates {
		if g.Name == name {
			return g, true
		}
	}
	return QualityGate{}, false
}

func suiteUsesInherit(suite []string) bool {
	for _, entry := range suite {
		if entry == "inherit" || strings.HasPrefix(entry, "+") {
			return true
		}
	}
	return false
}

func (c *Config) defaultSuiteForAgent(agentKey string) ([]quality.Gate, error) {
	selected, err := c.defaultSuiteGates(agentKey)
	if err != nil {
		return nil, err
	}
	return toRuntimeGates(selected), nil
}

func (c *Config) defaultSuiteGates(agentKey string) ([]QualityGate, error) {
	ac := c.Agents[agentKey]
	var ordered []QualityGate
	seen := make(map[string]bool)
	appendGate := func(g QualityGate) {
		if seen[g.Name] {
			return
		}
		seen[g.Name] = true
		ordered = append(ordered, g)
	}

	if len(c.Quality.BaselineSuite) > 0 {
		for _, name := range c.Quality.BaselineSuite {
			g, ok := c.Quality.GateByName(name)
			if !ok {
				return nil, fmt.Errorf("quality.baseline_suite references unknown gate %q", name)
			}
			if !gateAllowedForAgent(g, agentKey) {
				return nil, fmt.Errorf("agent %s: baseline gate %q is restricted to other agents", agentKey, name)
			}
			appendGate(g)
		}
	} else {
		var unrestricted []QualityGate
		for _, g := range c.Quality.Gates {
			if len(g.Agents) == 0 {
				unrestricted = append(unrestricted, g)
			}
		}
		slices.SortFunc(unrestricted, func(a, b QualityGate) int {
			if r := quality.LevelRank(a.Level) - quality.LevelRank(b.Level); r != 0 {
				return r
			}
			return strings.Compare(a.Name, b.Name)
		})
		for _, g := range unrestricted {
			appendGate(g)
		}
	}

	var extras []QualityGate
	for _, g := range c.Quality.Gates {
		if len(g.Agents) > 0 && slices.Contains(g.Agents, agentKey) {
			extras = append(extras, g)
		}
	}
	extras = append(extras, ac.QualityGates...)
	slices.SortFunc(extras, func(a, b QualityGate) int {
		if r := quality.LevelRank(a.Level) - quality.LevelRank(b.Level); r != 0 {
			return r
		}
		return strings.Compare(a.Name, b.Name)
	})
	for _, g := range extras {
		appendGate(g)
	}
	return ordered, nil
}

func (c *Config) inheritSuiteForAgent(agentKey string, suite []string) ([]quality.Gate, error) {
	selected, err := c.defaultSuiteGates(agentKey)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool, len(selected))
	for _, g := range selected {
		seen[g.Name] = true
	}
	for _, entry := range suite {
		if entry == "inherit" {
			continue
		}
		name := entry
		if strings.HasPrefix(entry, "+") {
			name = entry[1:]
		}
		if name == "" {
			return nil, fmt.Errorf("agent %s: invalid quality_suite entry %q", agentKey, entry)
		}
		if seen[name] {
			continue
		}
		g, ok := c.gateForAgent(agentKey, name)
		if !ok {
			return nil, fmt.Errorf("agent %s: quality_suite references unknown or disallowed gate %q", agentKey, name)
		}
		seen[name] = true
		selected = append(selected, g)
	}
	return toRuntimeGates(selected), nil
}

func toRuntimeGates(gates []QualityGate) []quality.Gate {
	out := make([]quality.Gate, 0, len(gates))
	for _, g := range gates {
		timeout := 5 * time.Minute
		if g.Timeout != "" {
			timeout, _ = time.ParseDuration(g.Timeout)
		}
		cmd, _ := g.ResolvedCmd()
		out = append(out, quality.Gate{
			Name:     g.Name,
			Kind:     g.KindOrDefault(),
			Level:    g.Level,
			Cmd:      cmd,
			Specs:    slices.Clone(g.Specs),
			Runner:   g.Runner,
			Blocking: g.Blocking,
			Timeout:  timeout,
			Retries:  g.Retries,
		})
	}
	return out
}

// KindOrDefault returns command when kind is unset.
func (g QualityGate) KindOrDefault() string {
	return quality.KindOrDefault(g.Kind)
}

// ResolvedCmd returns the shell command for this gate.
func (g QualityGate) ResolvedCmd() (string, error) {
	switch g.KindOrDefault() {
	case quality.KindCommand:
		cmd := strings.TrimSpace(g.Cmd)
		if cmd == "" {
			return "", fmt.Errorf("cmd is required for kind=command")
		}
		return cmd, nil
	case quality.KindBDD:
		if strings.TrimSpace(g.Cmd) != "" {
			return strings.TrimSpace(g.Cmd), nil
		}
		if len(g.Specs) == 0 {
			return "", fmt.Errorf("bdd gate requires specs (Gherkin feature paths) or an explicit cmd")
		}
		specs := strings.Join(g.Specs, " ")
		runner := g.Runner
		if runner == "" {
			runner = "godog"
		}
		switch runner {
		case "godog":
			return "godog " + specs, nil
		case "behave":
			return "behave " + specs, nil
		case "cucumber":
			return "npx cucumber-js " + specs, nil
		default:
			return "", fmt.Errorf("unknown bdd runner %q", runner)
		}
	default:
		return "", fmt.Errorf("unknown kind %q", g.Kind)
	}
}

// RuntimeQualityForAgent builds the provision-time runtime config blob.
func (c *Config) RuntimeQualityForAgent(agentKey string) (quality.RuntimeConfig, error) {
	gates, err := c.SuiteForAgent(agentKey)
	if err != nil {
		return quality.RuntimeConfig{}, err
	}
	rc := quality.RuntimeConfig{
		Enabled:     c.Quality != nil && c.Quality.Enabled,
		OnFailure:   c.Quality.OnFailureOrDefault(),
		MaxAttempts: c.Quality.MaxAttemptsOrDefault(),
		AgentKey:    agentKey,
		Project:     c.Project,
		Gates:       gates,
	}
	return rc, nil
}

func validateAgentQualitySuite(c *Config, agentKey string, suite []string) error {
	if len(suite) == 0 {
		return nil
	}
	if c.Quality == nil || !c.Quality.Enabled {
		return fmt.Errorf("agent %s: quality_suite set but quality.enabled is false", agentKey)
	}
	for _, entry := range suite {
		if entry == "inherit" {
			continue
		}
		name := entry
		if strings.HasPrefix(entry, "+") {
			name = entry[1:]
		}
		if name == "" {
			return fmt.Errorf("agent %s: invalid quality_suite entry %q", agentKey, entry)
		}
		if _, ok := c.gateForAgent(agentKey, name); !ok {
			return fmt.Errorf("agent %s: quality_suite references unknown or disallowed gate %q", agentKey, name)
		}
	}
	return nil
}

func validateAgentQualityGates(c *Config, agentKey string, gates []QualityGate) error {
	if len(gates) == 0 {
		return nil
	}
	if c.Quality == nil || !c.Quality.Enabled {
		return fmt.Errorf("agent %s: quality_gates set but quality.enabled is false", agentKey)
	}
	names := make(map[string]bool, len(gates))
	for i, g := range gates {
		if g.Name == "" {
			return fmt.Errorf("agent %s: quality_gates[%d]: name is required", agentKey, i)
		}
		if names[g.Name] {
			return fmt.Errorf("agent %s: duplicate quality_gates name %q", agentKey, g.Name)
		}
		names[g.Name] = true
		if _, ok := c.Quality.GateByName(g.Name); ok {
			return fmt.Errorf("agent %s: quality_gates name %q conflicts with global gate", agentKey, g.Name)
		}
		if !quality.ValidLevel(g.Level) {
			return fmt.Errorf("agent %s: quality_gates[%d] (%s): unknown level %q", agentKey, i, g.Name, g.Level)
		}
		if !quality.ValidKind(g.Kind) {
			return fmt.Errorf("agent %s: quality_gates[%d] (%s): unknown kind %q", agentKey, i, g.Name, g.Kind)
		}
		if _, err := g.ResolvedCmd(); err != nil {
			return fmt.Errorf("agent %s: quality_gates[%d] (%s): %w", agentKey, i, g.Name, err)
		}
	}
	return nil
}
