package githubwatch

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/jahwag/clem/internal/config"
)

func baseCfg() *config.Config {
	return &config.Config{
		Project: "test",
		Coordination: config.Coordination{
			Backend:    "github",
			GithubRepo: "acme/tasks",
			Channels: map[string]string{
				"tasks":  "clem:todo",
				"alerts": "12",
			},
		},
		Agents: map[string]config.AgentConfig{
			"worker": {
				Name:      "Worker",
				Model:     "claude-opus-4-7",
				Iteration: "1m",
				Prompt:    "go",
			},
		},
	}
}

func TestGenerateScript_PollsGitHubAPI(t *testing.T) {
	s := GenerateScript(baseCfg(), "worker")
	for _, want := range []string{
		`REPO="acme/tasks"`,
		`LABEL="clem:todo"`,
		`LABEL_ENC="clem%3Atodo"`,
		`api.github.com/repos/${REPO}/issues`,
		`If-None-Match`,
		`tmux send-keys -t "$AGENT_KEY"`,
		`POLL_INTERVAL=60`,
		`DEBOUNCE=5`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("script missing %q:\n%s", want, s)
		}
	}
}

func TestGenerateScript_EgressProxyExport(t *testing.T) {
	cfg := baseCfg()
	cfg.Egress.Enabled = true
	s := GenerateScript(cfg, "worker")
	if !strings.Contains(s, `export HTTPS_PROXY=http://127.0.0.1:8888`) {
		t.Fatalf("expected HTTPS_PROXY in watch script:\n%s", s)
	}
}

func TestGenerateService_JoinsAgentNamespace(t *testing.T) {
	s := GenerateService(baseCfg(), "worker")
	for _, want := range []string{
		"JoinsNamespaceOf=clem-test-worker.service",
		"BindsTo=clem-test-worker.service",
		"ExecStart=/home/test-worker/.local/bin/clem-github-watch.sh",
		"User=test-worker",
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("service missing %q:\n%s", want, s)
		}
	}
}

func TestGenerateScript_UsesHadStateFileForWakeDiff(t *testing.T) {
	s := GenerateScript(baseCfg(), "worker")
	for _, want := range []string{
		`HAD_STATE_FILE=0`,
		`HAD_STATE_FILE=1`,
		`if [ "$HAD_STATE_FILE" -eq 1 ]; then`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("script missing %q:\n%s", want, s)
		}
	}
	if strings.Contains(s, `if [ -n "$OLD_IDS" ]; then`) {
		t.Fatalf("script still uses OLD_IDS guard for wake diff:\n%s", s)
	}
}

func TestGenerateService_EgressPipelockDeps(t *testing.T) {
	cfg := baseCfg()
	cfg.Egress.Enabled = true
	s := GenerateService(cfg, "worker")
	for _, want := range []string{
		"After=clem-pipelock-test.service",
		"Wants=clem-pipelock-test.service",
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("service missing %q:\n%s", want, s)
		}
	}
}

// wakeDiffScript mirrors the poll_once wake guard from the generated watcher.
const wakeDiffScript = `
HAD_STATE_FILE=$1
OLD_IDS=$2
NEW_IDS=$3
if [ "$HAD_STATE_FILE" -eq 1 ]; then
    if comm -13 <(echo "$OLD_IDS" | tr ' ' '\n' | sort -n) <(echo "$NEW_IDS" | tr ' ' '\n' | sort -n) | grep -q .; then
        echo WAKE
    fi
fi
`

func TestWakeDiff_EmptyToNonemptyWithPriorState(t *testing.T) {
	out, err := exec.Command("bash", "-c", wakeDiffScript, "bash", "1", "", "42").Output()
	if err != nil {
		t.Fatalf("bash: %v", err)
	}
	if !strings.Contains(string(out), "WAKE") {
		t.Fatalf("expected wake on empty→non-empty with prior state, got %q", out)
	}
}

func TestWakeDiff_NoWakeOnFirstPoll(t *testing.T) {
	out, err := exec.Command("bash", "-c", wakeDiffScript, "bash", "0", "", "42").Output()
	if err != nil {
		t.Fatalf("bash: %v", err)
	}
	if strings.Contains(string(out), "WAKE") {
		t.Fatalf("first poll should not wake backlog, got %q", out)
	}
}

func TestWakeDiff_NoWakeWhenUnchanged(t *testing.T) {
	out, err := exec.Command("bash", "-c", wakeDiffScript, "bash", "1", "1 2", "1 2").Output()
	if err != nil {
		t.Fatalf("bash: %v", err)
	}
	if strings.Contains(string(out), "WAKE") {
		t.Fatalf("unchanged IDs should not wake, got %q", out)
	}
}
