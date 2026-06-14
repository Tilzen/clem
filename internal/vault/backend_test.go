package vault

import (
	"errors"
	"os/exec"
	"strings"
	"testing"

	"github.com/jahwag/clem/internal/config"
)

// stubSource returns a fixed map (or error) regardless of agent/buckets.
type stubSource struct {
	name    string
	secrets map[string]string
	err     error
}

func (s stubSource) Name() string { return s.name }

func (s stubSource) Decrypt(string, []string) (map[string]string, error) {
	return s.secrets, s.err
}

func TestSources_DefaultsToSops(t *testing.T) {
	srcs, err := Sources(nil)
	if err != nil {
		t.Fatalf("Sources(nil): %v", err)
	}
	if len(srcs) != 1 {
		t.Fatalf("expected 1 implicit source, got %d", len(srcs))
	}
	if srcs[0].Name() != "local" {
		t.Errorf("implicit source name = %q, want local", srcs[0].Name())
	}
	if _, ok := srcs[0].(sopsSource); !ok {
		t.Errorf("implicit source is %T, want sopsSource", srcs[0])
	}
}

func TestSources_EmptyTypeIsSops(t *testing.T) {
	srcs, err := Sources([]config.VaultSource{{Name: "main"}})
	if err != nil {
		t.Fatalf("Sources: %v", err)
	}
	if _, ok := srcs[0].(sopsSource); !ok {
		t.Errorf("empty type resolved to %T, want sopsSource", srcs[0])
	}
}

func TestSources_UnknownTypeErrors(t *testing.T) {
	_, err := Sources([]config.VaultSource{{Name: "x", Type: "hashicorp"}})
	if err == nil {
		t.Fatal("expected error for unknown source type")
	}
	if !strings.Contains(err.Error(), "hashicorp") || !strings.Contains(err.Error(), "sops") {
		t.Errorf("error should name the bad type and the valid set, got: %v", err)
	}
}

func TestMergeSources_LaterSourceWins(t *testing.T) {
	srcs := []Source{
		stubSource{name: "a", secrets: map[string]string{"shared.KEY": "from-a", "shared.ONLY_A": "a"}},
		stubSource{name: "b", secrets: map[string]string{"shared.KEY": "from-b", "shared.ONLY_B": "b"}},
	}
	got, err := mergeSources(srcs, "agent", []string{"shared"})
	if err != nil {
		t.Fatalf("mergeSources: %v", err)
	}
	want := map[string]string{"shared.KEY": "from-b", "shared.ONLY_A": "a", "shared.ONLY_B": "b"}
	if len(got) != len(want) {
		t.Fatalf("got %d keys, want %d: %v", len(got), len(want), got)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("key %s = %q, want %q", k, got[k], v)
		}
	}
}

func TestMergeSources_ErrorNamesSource(t *testing.T) {
	boom := errors.New("decrypt failed")
	srcs := []Source{stubSource{name: "infra", err: boom}}
	_, err := mergeSources(srcs, "agent", nil)
	if err == nil {
		t.Fatal("expected error from failing source")
	}
	if !strings.Contains(err.Error(), "infra") {
		t.Errorf("error should name the source, got: %v", err)
	}
	if !errors.Is(err, boom) {
		t.Errorf("error should wrap the source error, got: %v", err)
	}
}

// TestDecryptAgentSecrets_MatchesDecryptForAgent pins the back-compat
// guarantee: with no backends configured, the dispatch path returns exactly
// what the direct sops call returns.
func TestDecryptAgentSecrets_MatchesDecryptForAgent(t *testing.T) {
	requireSopsAndAge(t)
	if _, err := exec.LookPath("yq"); err != nil {
		t.Skip("yq not on PATH — skipping integration test")
	}
	cleanup := setupVaultDir(t)
	defer cleanup()

	if err := Set("shared", "API_KEY=abc123"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := Set("extra", "API_KEY=zzz"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	direct, err := DecryptForAgent("worker", []string{"shared", "extra"})
	if err != nil {
		t.Fatalf("DecryptForAgent: %v", err)
	}
	dispatched, err := DecryptAgentSecrets(nil, "worker", []string{"shared", "extra"})
	if err != nil {
		t.Fatalf("DecryptAgentSecrets: %v", err)
	}
	if len(direct) != len(dispatched) {
		t.Fatalf("key count differs: direct=%v dispatched=%v", direct, dispatched)
	}
	for k, v := range direct {
		if dispatched[k] != v {
			t.Errorf("key %s: direct=%q dispatched=%q", k, v, dispatched[k])
		}
	}
}
