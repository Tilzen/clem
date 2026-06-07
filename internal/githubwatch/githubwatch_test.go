package githubwatch

import (
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
