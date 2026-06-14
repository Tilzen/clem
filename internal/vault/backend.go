package vault

import (
	"fmt"
	"strings"

	"github.com/jahwag/clem/internal/config"
)

// Source is a secret source backend: it materializes the named buckets for an
// agent into a qualified map ("bucket.KEY" -> value, see DecryptForAgent).
// sops is the only implementation today; Infisical Agent lands behind the
// same interface (#174). Sources are about where secrets come from — the
// env/agent-vault split (config.VaultBackend.Backend) stays orthogonal and
// decides how the merged result is materialized for the agent.
type Source interface {
	// Name returns the configured instance name (e.g. "local"), used to
	// qualify errors and, later, bucket refs.
	Name() string
	// Decrypt returns the qualified secrets for an agent, merging the named
	// buckets in order with later buckets winning on key conflicts.
	Decrypt(agentKey string, buckets []string) (map[string]string, error)
}

// sopsSource adapts the package-level sops helpers to the Source interface.
type sopsSource struct{ name string }

func (s sopsSource) Name() string { return s.name }

func (s sopsSource) Decrypt(agentKey string, buckets []string) (map[string]string, error) {
	return DecryptForAgent(agentKey, buckets)
}

// Sources resolves the configured vault source backends into Source
// implementations, in list order. An empty config yields the implicit sops
// default, so existing clem.yaml files keep working unchanged. Unknown types
// are already rejected at config.Load; the error here is a backstop for
// callers constructing config.VaultSource values directly.
func Sources(cfgs []config.VaultSource) ([]Source, error) {
	if len(cfgs) == 0 {
		cfgs = []config.VaultSource{{Name: "local", Type: "sops"}}
	}
	out := make([]Source, 0, len(cfgs))
	for _, c := range cfgs {
		s, err := knownSource(c)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}

// knownSource maps one config entry to its implementation. This switch must
// handle every type in config.ValidVaultSourceTypes; Load() validates against
// that set, so the error here is only a backstop for callers constructing
// config.VaultSource values directly.
func knownSource(c config.VaultSource) (Source, error) {
	switch c.Type {
	case "", "sops":
		return sopsSource{name: c.Name}, nil
	default:
		return nil, fmt.Errorf("unknown vault source type %q (valid: %s)", c.Type, strings.Join(config.ValidVaultSourceTypes, ", "))
	}
}

// DecryptAgentSecrets merges the qualified secrets for an agent across the
// configured source backends. Merge order is list order with later sources
// winning on key conflicts — the same last-wins semantics as the bucket merge
// inside each source. With the default sops-only config this is exactly
// DecryptForAgent.
func DecryptAgentSecrets(cfgs []config.VaultSource, agentKey string, buckets []string) (map[string]string, error) {
	srcs, err := Sources(cfgs)
	if err != nil {
		return nil, err
	}
	return mergeSources(srcs, agentKey, buckets)
}

func mergeSources(srcs []Source, agentKey string, buckets []string) (map[string]string, error) {
	merged := make(map[string]string)
	for _, s := range srcs {
		m, err := s.Decrypt(agentKey, buckets)
		if err != nil {
			return nil, fmt.Errorf("vault source %s: %w", s.Name(), err)
		}
		for k, v := range m {
			merged[k] = v
		}
	}
	return merged, nil
}
