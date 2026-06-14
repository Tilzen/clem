package quality_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jahwag/clem/internal/quality"
)

func TestRunSuite_FailFastOnBlocking(t *testing.T) {
	dir := t.TempDir()
	gates := []quality.Gate{
		{Name: "build", Level: "build", Cmd: "exit 1", Blocking: true},
		{Name: "unit", Level: "unit", Cmd: "exit 0", Blocking: true},
	}
	v := quality.RunSuite(dir, gates, "task-1", 1)
	if v.Pass {
		t.Fatal("expected fail")
	}
	if len(v.Results) != 1 {
		t.Fatalf("expected fail-fast after 1 gate, got %d results", len(v.Results))
	}
}

func TestRunSuite_RunsAllWhenNonBlockingFails(t *testing.T) {
	dir := t.TempDir()
	gates := []quality.Gate{
		{Name: "lint", Level: "lint", Cmd: "exit 1", Blocking: false},
		{Name: "unit", Level: "unit", Cmd: "exit 0", Blocking: true},
	}
	v := quality.RunSuite(dir, gates, "task-1", 1)
	if !v.Pass {
		t.Fatal("expected pass when only advisory gate fails")
	}
	if len(v.Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(v.Results))
	}
}

func TestRunIteration_FeedbackAndAttempts(t *testing.T) {
	home := t.TempDir()
	work := filepath.Join(home, "proj")
	if err := os.MkdirAll(work, 0755); err != nil {
		t.Fatal(err)
	}
	claudeLocal := filepath.Join(work, "CLAUDE.local.md")
	if err := os.WriteFile(claudeLocal, []byte("# local\n"), 0644); err != nil {
		t.Fatal(err)
	}
	rc := quality.RuntimeConfig{
		Enabled:     true,
		OnFailure:   "feedback",
		MaxAttempts: 3,
		AgentKey:    "lead",
		Project:     "proj",
		Gates: []quality.Gate{
			{Name: "unit", Level: "unit", Cmd: "exit 1", Blocking: true},
		},
	}
	code, err := quality.RunIteration(home, work, rc)
	if code != 1 || err == nil {
		t.Fatalf("expected code 1 with error, got code=%d err=%v", code, err)
	}
	feedback, err := os.ReadFile(quality.FeedbackPath(home))
	if err != nil {
		t.Fatalf("feedback file: %v", err)
	}
	if !strings.Contains(string(feedback), "[quality]") {
		t.Fatalf("unexpected feedback: %s", feedback)
	}
	content, _ := os.ReadFile(claudeLocal)
	if !strings.Contains(string(content), quality.FeedbackStartMarker) {
		t.Fatal("CLAUDE.local.md missing feedback block")
	}
}

func TestRunIteration_PassClearsFeedback(t *testing.T) {
	home := t.TempDir()
	work := filepath.Join(home, "proj")
	if err := os.MkdirAll(work, 0755); err != nil {
		t.Fatal(err)
	}
	claudeLocal := filepath.Join(work, "CLAUDE.local.md")
	if err := os.WriteFile(claudeLocal, []byte(quality.FeedbackStartMarker+"\nold\n"+quality.FeedbackEndMarker), 0644); err != nil {
		t.Fatal(err)
	}
	rc := quality.RuntimeConfig{
		Enabled:   true,
		OnFailure: "feedback",
		AgentKey:  "lead",
		Gates: []quality.Gate{
			{Name: "unit", Level: "unit", Cmd: "true", Blocking: true},
		},
	}
	code, err := quality.RunIteration(home, work, rc)
	if code != 0 || err != nil {
		t.Fatalf("expected pass, got code=%d err=%v", code, err)
	}
	if _, err := os.Stat(quality.FeedbackPath(home)); !os.IsNotExist(err) {
		t.Fatal("feedback file should be removed on pass")
	}
	content, _ := os.ReadFile(claudeLocal)
	if strings.Contains(string(content), quality.FeedbackStartMarker) {
		t.Fatalf("CLAUDE.local.md should not contain feedback block after pass: %q", content)
	}
}

func TestRunPrePush_BlockPushMode(t *testing.T) {
	home := t.TempDir()
	work := t.TempDir()
	rc := quality.RuntimeConfig{
		Enabled:   true,
		OnFailure: "block-push",
		AgentKey:  "lead",
		Gates: []quality.Gate{
			{Name: "unit", Level: "unit", Cmd: "exit 1", Blocking: true},
		},
	}
	if err := quality.RunPrePush(home, work, rc); err == nil {
		t.Fatal("expected pre-push failure")
	}
}
