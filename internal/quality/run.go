package quality

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const defaultGateTimeout = 5 * time.Minute

// RunSuite executes gates in order with fail-fast on blocking failures.
func RunSuite(workdir string, gates []Gate, taskID string, attempt int) Verdict {
	v := Verdict{
		TaskID:    taskID,
		Attempt:   attempt,
		Timestamp: time.Now().UTC(),
		Pass:      true,
	}
	for _, gate := range gates {
		result := runGateWithRetries(workdir, gate)
		v.Results = append(v.Results, result)
		if !result.Pass && gate.Blocking {
			v.Pass = false
			v.BlockingFailed = true
			break
		}
	}
	if v.BlockingFailed {
		v.Pass = false
	}
	return v
}

func runGateWithRetries(workdir string, gate Gate) GateResult {
	retries := gate.Retries
	if retries < 0 {
		retries = 0
	}
	var last GateResult
	for i := 0; i <= retries; i++ {
		last = runGate(workdir, gate, i+1)
		if last.Pass {
			return last
		}
	}
	return last
}

func runGate(workdir string, gate Gate, attemptNum int) GateResult {
	timeout := gate.Timeout
	if timeout <= 0 {
		timeout = defaultGateTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	start := time.Now()
	cmd := exec.CommandContext(ctx, "bash", "-lc", gate.Cmd)
	cmd.Dir = workdir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	duration := time.Since(start)

	result := GateResult{
		Name:       gate.Name,
		Kind:       KindOrDefault(gate.Kind),
		Level:      gate.Level,
		Specs:      gate.Specs,
		Blocking:   gate.Blocking,
		Attempt:    attemptNum,
		DurationMS: duration.Milliseconds(),
	}
	if err == nil {
		result.Pass = true
		result.ExitCode = 0
	} else if ctx.Err() == context.DeadlineExceeded {
		result.Error = fmt.Sprintf("timeout after %s", timeout)
		result.ExitCode = 124
	} else if exitErr, ok := err.(*exec.ExitError); ok {
		result.ExitCode = exitErr.ExitCode()
	} else {
		result.Error = err.Error()
		result.ExitCode = 1
	}
	combined := strings.TrimSpace(stdout.String() + "\n" + stderr.String())
	result.Output = truncateOutput(combined, 8192)
	return result
}

func truncateOutput(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}
