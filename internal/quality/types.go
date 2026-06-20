package quality

import "time"

// Canonical gate levels, cheapest to most expensive.
var LevelOrder = []string{
	"build", "typecheck", "lint", "unit", "integration", "acceptance", "custom",
}

// RuntimeConfig is the per-agent quality configuration written at provision
// time to ~/.clem/quality.json for runner and pre-push consumption.
type RuntimeConfig struct {
	Enabled     bool   `json:"enabled"`
	OnFailure   string `json:"on_failure"`
	MaxAttempts int    `json:"max_attempts"`
	AgentKey    string `json:"agent_key"`
	Project     string `json:"project"`
	Gates       []Gate `json:"gates"`
	AlertScript string `json:"alert_script,omitempty"`
}

// Gate is a single deterministic quality check.
type Gate struct {
	Name     string        `json:"name"`
	Kind     string        `json:"kind,omitempty"` // command (default) | bdd
	Level    string        `json:"level"`
	Cmd      string        `json:"cmd"`
	Specs    []string      `json:"specs,omitempty"`  // Gherkin paths for kind=bdd
	Runner   string        `json:"runner,omitempty"` // godog | behave | cucumber
	Blocking bool          `json:"blocking"`
	Timeout  time.Duration `json:"timeout"`
	Retries  int           `json:"retries"`
}

// GateResult captures one gate execution outcome.
type GateResult struct {
	Name       string   `json:"name"`
	Kind       string   `json:"kind,omitempty"`
	Level      string   `json:"level"`
	Specs      []string `json:"specs,omitempty"`
	Blocking   bool     `json:"blocking"`
	Pass       bool     `json:"pass"`
	ExitCode   int      `json:"exit_code"`
	DurationMS int64    `json:"duration_ms"`
	Output     string   `json:"output,omitempty"`
	Error      string   `json:"error,omitempty"`
	Attempt    int      `json:"attempt"`
}

// Verdict is the aggregated result of a suite run.
type Verdict struct {
	Pass           bool         `json:"pass"`
	BlockingFailed bool         `json:"blocking_failed"`
	Results        []GateResult `json:"results"`
	TaskID         string       `json:"task_id"`
	Attempt        int          `json:"attempt"`
	Timestamp      time.Time    `json:"timestamp"`
}

// JSONLEntry is one append-only metrics record.
type JSONLEntry struct {
	TS         time.Time `json:"ts"`
	TaskID     string    `json:"task_id"`
	Attempt    int       `json:"attempt"`
	Gate       string    `json:"gate"`
	Pass       bool      `json:"pass"`
	DurationMS int64     `json:"duration_ms"`
	ExitCode   int       `json:"exit_code"`
	Blocking   bool      `json:"blocking"`
}

// State tracks per-task closed-loop attempts.
type State struct {
	TaskID   string `json:"task_id"`
	Attempts int    `json:"attempts"`
	Blocked  bool   `json:"blocked"`
}
