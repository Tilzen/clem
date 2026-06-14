package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jahwag/clem/internal/quality"
	"github.com/spf13/cobra"
)

func TestExecuteQualityRun_ClosedLoop(t *testing.T) {
	home := t.TempDir()
	work := filepath.Join(home, "demo")
	if err := os.MkdirAll(work, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(work, "CLAUDE.local.md"), []byte("# ctx\n"), 0644); err != nil {
		t.Fatal(err)
	}
	rc := quality.RuntimeConfig{
		Enabled:     true,
		OnFailure:   "feedback",
		MaxAttempts: 3,
		AgentKey:    "lead",
		Project:     "demo",
		Gates: []quality.Gate{
			{Name: "unit", Level: "unit", Cmd: "test -f .quality_ok", Blocking: true},
		},
	}
	if err := quality.WriteRuntimeConfig(home, rc); err != nil {
		t.Fatal(err)
	}

	cmd := &cobra.Command{}
	cmd.Flags().String("home", home, "")
	cmd.Flags().String("workdir", work, "")
	if err := cmd.Flags().Set("home", home); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Flags().Set("workdir", work); err != nil {
		t.Fatal(err)
	}

	code, err := executeQualityRun(cmd)
	if err != nil {
		t.Fatal(err)
	}
	if code != 1 {
		t.Fatalf("fail without marker: code=%d", code)
	}

	if err := os.WriteFile(filepath.Join(work, ".quality_ok"), []byte("1"), 0644); err != nil {
		t.Fatal(err)
	}
	code, err = executeQualityRun(cmd)
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 {
		t.Fatalf("pass with marker: code=%d", code)
	}
}

func TestLoadConfigSkipsQualitySubcommands(t *testing.T) {
	tmp := t.TempDir()
	prev, _ := os.Getwd()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(prev) }()

	qualityParent := &cobra.Command{Use: "quality"}
	if err := loadConfig(qualityParent, nil); err != nil {
		t.Errorf("loadConfig(quality): %v", err)
	}
	for _, name := range []string{"run", "pre-push"} {
		sub := &cobra.Command{Use: name}
		qualityParent.AddCommand(sub)
		if err := loadConfig(sub, nil); err != nil {
			t.Errorf("loadConfig(quality %s): %v", name, err)
		}
	}
}
