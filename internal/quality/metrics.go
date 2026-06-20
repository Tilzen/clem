package quality

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func deferClose(f *os.File, err *error) {
	if cerr := f.Close(); cerr != nil && *err == nil {
		*err = cerr
	}
}

// AppendJSONL records each gate result from a verdict run.
func AppendJSONL(homeDir, agentKey string, v Verdict) (err error) {
	path := JSONLPath(homeDir, agentKey)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer func() {
		deferClose(f, &err)
	}()
	for _, r := range v.Results {
		entry := JSONLEntry{
			TS:         v.Timestamp,
			TaskID:     v.TaskID,
			Attempt:    v.Attempt,
			Gate:       r.Name,
			Pass:       r.Pass,
			DurationMS: r.DurationMS,
			ExitCode:   r.ExitCode,
			Blocking:   r.Blocking,
		}
		data, err := json.Marshal(entry)
		if err != nil {
			return err
		}
		if _, err := f.Write(append(data, '\n')); err != nil {
			return err
		}
	}
	return nil
}

// AgentSummary is the latest quality snapshot for one agent.
type AgentSummary struct {
	AgentKey    string
	LastPass    bool
	GatesPass   int
	GatesTotal  int
	LastAttempt int
	LastGate    string
	LastTS      time.Time
	PassRate    float64
}

// GateStats aggregates metrics for one gate name.
type GateStats struct {
	Name     string
	Runs     int
	Passes   int
	PassRate float64
	AvgMS    int64
}

// ReadAgentSummary parses the last verdict line-group from JSONL.
func ReadAgentSummary(homeDir, agentKey string) (sum AgentSummary, err error) {
	path := JSONLPath(homeDir, agentKey)
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return AgentSummary{AgentKey: agentKey}, nil
		}
		return AgentSummary{}, err
	}
	defer deferClose(f, &err)

	var entries []JSONLEntry
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var e JSONLEntry
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			continue
		}
		entries = append(entries, e)
	}
	if len(entries) == 0 {
		return AgentSummary{AgentKey: agentKey}, nil
	}

	// Take the last run by attempt number (all gates in one suite share attempt).
	lastAttempt := entries[len(entries)-1].Attempt
	var lastRun []JSONLEntry
	for i := len(entries) - 1; i >= 0; i-- {
		if entries[i].Attempt != lastAttempt {
			break
		}
		lastRun = append([]JSONLEntry{entries[i]}, lastRun...)
	}
	lastTS := entries[len(entries)-1].TS
	sum = AgentSummary{AgentKey: agentKey, LastTS: lastTS, LastAttempt: lastAttempt}
	for _, e := range lastRun {
		sum.GatesTotal++
		if e.Pass {
			sum.GatesPass++
		} else {
			sum.LastPass = false
			sum.LastGate = e.Gate
		}
	}
	if sum.GatesTotal > 0 && sum.GatesPass == sum.GatesTotal {
		sum.LastPass = true
	}
	if len(lastRun) > 0 && sum.LastGate == "" {
		sum.LastGate = lastRun[len(lastRun)-1].Gate
	}

	// Pass rate across all entries.
	passes, total := 0, 0
	for _, e := range entries {
		total++
		if e.Pass {
			passes++
		}
	}
	if total > 0 {
		sum.PassRate = float64(passes) / float64(total)
	}
	return sum, err
}

// AggregateGateStats computes per-gate stats from JSONL.
func AggregateGateStats(homeDir, agentKey string) (out []GateStats, err error) {
	path := JSONLPath(homeDir, agentKey)
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer deferClose(f, &err)

	byGate := map[string]*GateStats{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var e JSONLEntry
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			continue
		}
		st, ok := byGate[e.Gate]
		if !ok {
			st = &GateStats{Name: e.Gate}
			byGate[e.Gate] = st
		}
		st.Runs++
		if e.Pass {
			st.Passes++
		}
		st.AvgMS += e.DurationMS
	}
	out = make([]GateStats, 0, len(byGate))
	for _, st := range byGate {
		if st.Runs > 0 {
			st.PassRate = float64(st.Passes) / float64(st.Runs)
			st.AvgMS /= int64(st.Runs)
		}
		out = append(out, *st)
	}
	return out, err
}

// LoadRuntimeConfig reads ~/.clem/quality.json.
func LoadRuntimeConfig(homeDir string) (RuntimeConfig, error) {
	data, err := os.ReadFile(RuntimeConfigPath(homeDir))
	if err != nil {
		return RuntimeConfig{}, fmt.Errorf("reading quality config: %w", err)
	}
	var rc RuntimeConfig
	if err := json.Unmarshal(data, &rc); err != nil {
		return RuntimeConfig{}, fmt.Errorf("parsing quality config: %w", err)
	}
	return rc, nil
}

// WriteRuntimeConfig writes ~/.clem/quality.json.
func WriteRuntimeConfig(homeDir string, rc RuntimeConfig) error {
	if err := os.MkdirAll(filepath.Join(homeDir, ".clem"), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(rc, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(RuntimeConfigPath(homeDir), data, 0600)
}

// RunIteration executes the full post-session quality loop side effects.
// Returns exit code:
//
//	0 on pass (or advisory/disabled no-op),
//	1 on a non-final blocking failure (feedback injected, agent may retry),
//	2 on the transition into blocked — max attempts just exhausted; the runner
//	  fires the [BLOCKED] alert once and feedback is cleared (the alert is the
//	  human signal, the agent cannot self-recover),
//	3 on an already-blocked iteration — a quiet no-op that does NOT re-run gates,
//	  re-alert, or re-inject feedback (the human was alerted on the transition).
func RunIteration(homeDir, workdir string, rc RuntimeConfig) (int, error) {
	if !rc.Enabled || len(rc.Gates) == 0 {
		return 0, nil
	}
	taskID := CurrentTaskID(homeDir)
	state, err := ResetAttemptsIfTaskChanged(homeDir, taskID)
	if err != nil {
		return 1, err
	}
	if state.Blocked && rc.OnFailure != "block-push" {
		// Already blocked: quiet no-op. Return 3 (not 2) so the runner does not
		// re-fire the [BLOCKED] alert every loop. Feedback was already cleared
		// at the transition, so nothing to re-inject here.
		return 3, fmt.Errorf("task %q is blocked after exhausting quality attempts", taskID)
	}

	if rc.OnFailure == "block-push" {
		verdict := RunSuite(workdir, rc.Gates, taskID, 0)
		if err := AppendJSONL(homeDir, rc.AgentKey, verdict); err != nil {
			return 1, err
		}
		return 0, nil
	}

	attempt := state.Attempts + 1
	verdict := RunSuite(workdir, rc.Gates, taskID, attempt)
	if err := AppendJSONL(homeDir, rc.AgentKey, verdict); err != nil {
		return 1, err
	}

	claudeLocal := filepath.Join(workdir, "CLAUDE.local.md")
	if verdict.Pass {
		state.Attempts = 0
		state.Blocked = false
		if err := ClearFeedback(homeDir, claudeLocal); err != nil {
			return 1, err
		}
		if err := SaveState(homeDir, state); err != nil {
			return 1, err
		}
		return 0, nil
	}

	if rc.OnFailure == "advisory" {
		return 0, nil
	}

	state.Attempts = attempt
	feedback := FormatFeedback(verdict, attempt, rc.MaxAttempts)
	if err := WriteFeedbackFile(homeDir, feedback); err != nil {
		return 1, err
	}
	if err := InjectFeedbackBlock(claudeLocal, feedback); err != nil && !os.IsNotExist(err) {
		return 1, err
	}

	if rc.MaxAttempts > 0 && attempt >= rc.MaxAttempts {
		state.Blocked = true
		// Clear the loop feedback at the transition: the agent has exhausted its
		// attempts and cannot self-recover, so re-prepending the feedback every
		// subsequent iteration is noise. The [BLOCKED] alert is the human signal.
		if err := ClearFeedback(homeDir, claudeLocal); err != nil {
			return 2, err
		}
		if err := SaveState(homeDir, state); err != nil {
			return 2, err
		}
		return 2, fmt.Errorf("quality gates failed %d times for task %q — marked blocked", attempt, taskID)
	}
	if err := SaveState(homeDir, state); err != nil {
		return 1, err
	}
	return 1, fmt.Errorf("quality gates failed (attempt %d/%d)", attempt, rc.MaxAttempts)
}

// RunPrePush runs blocking gates for block-push mode.
func RunPrePush(homeDir, workdir string, rc RuntimeConfig) error {
	if !rc.Enabled || rc.OnFailure != "block-push" {
		return nil
	}
	var blocking []Gate
	for _, g := range rc.Gates {
		if g.Blocking {
			blocking = append(blocking, g)
		}
	}
	if len(blocking) == 0 {
		return nil
	}
	v := RunSuite(workdir, blocking, "pre-push", 0)
	if err := AppendJSONL(homeDir, rc.AgentKey, v); err != nil {
		return err
	}
	if !v.Pass {
		return fmt.Errorf("quality pre-push gate(s) failed")
	}
	return nil
}
