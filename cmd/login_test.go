package cmd

import (
	"strings"
	"testing"

	"github.com/jahwag/clem/internal/config"
)

// selectAgents honours the [agent...] positional args of clem login:
// no args targets every agent, named args target only those agents,
// and an unknown key fails instead of silently logging in everyone.
func TestSelectAgents(t *testing.T) {
	all := map[string]config.AgentConfig{
		"lead":   {Name: "Lead"},
		"worker": {Name: "Worker"},
	}

	t.Run("no-args-returns-all", func(t *testing.T) {
		got, err := selectAgents(all, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != len(all) {
			t.Errorf("got %d agents, want %d", len(got), len(all))
		}
	})

	t.Run("single-key-filters", func(t *testing.T) {
		got, err := selectAgents(all, []string{"lead"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("got %d agents, want 1", len(got))
		}
		if _, ok := got["lead"]; !ok {
			t.Errorf("selected set missing %q: %v", "lead", got)
		}
	})

	t.Run("multiple-keys-filter", func(t *testing.T) {
		got, err := selectAgents(all, []string{"lead", "worker"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 2 {
			t.Errorf("got %d agents, want 2", len(got))
		}
	})

	t.Run("unknown-key-errors", func(t *testing.T) {
		_, err := selectAgents(all, []string{"lead", "nosuch"})
		if err == nil {
			t.Fatal("expected error for unknown agent key, got nil")
		}
		if !strings.Contains(err.Error(), "nosuch") {
			t.Errorf("error %q does not name the unknown key", err)
		}
	})
}
