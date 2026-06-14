package quality

import "strings"

// Gate kinds — extensible check types. "command" is the default escape hatch;
// "bdd" binds living Gherkin specs to executable acceptance gates (SDD/BDD).
const (
	KindCommand = "command"
	KindBDD     = "bdd"
)

var ValidKinds = []string{KindCommand, KindBDD}

// ValidKind reports whether kind is known (empty counts as command).
func ValidKind(kind string) bool {
	if kind == "" {
		return true
	}
	for _, k := range ValidKinds {
		if k == kind {
			return true
		}
	}
	return false
}

// KindOrDefault normalizes an empty kind to command.
func KindOrDefault(kind string) string {
	if kind == "" {
		return KindCommand
	}
	return kind
}

// ExtractBDDHints pulls Feature/Scenario/step failure lines from BDD runner
// output so agents get deterministic, spec-grounded feedback — not just a
// generic exit code.
func ExtractBDDHints(output string) string {
	var hints []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		lower := strings.ToLower(line)
		switch {
		case strings.HasPrefix(lower, "feature:"),
			strings.HasPrefix(lower, "scenario:"),
			strings.HasPrefix(lower, "scenario outline:"),
			strings.Contains(lower, "failed steps:"),
			strings.Contains(lower, "undefined scenarios:"),
			strings.Contains(lower, "ambiguous steps:"),
			strings.HasPrefix(lower, "  ✖"),
			strings.HasPrefix(lower, "  ×"),
			strings.Contains(lower, "assertionerror"),
			strings.Contains(lower, "expected"),
			strings.Contains(lower, "gherkin"):
			hints = append(hints, line)
		}
		if len(hints) >= 24 {
			break
		}
	}
	if len(hints) == 0 {
		return ""
	}
	return strings.Join(hints, "\n")
}
