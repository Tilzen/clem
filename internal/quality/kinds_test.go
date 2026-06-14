package quality_test

import (
	"strings"
	"testing"

	"github.com/jahwag/clem/internal/quality"
)

func TestExtractBDDHints(t *testing.T) {
	out := quality.ExtractBDDHints(`Feature: Login
  Scenario: valid credentials
    ✖ expected 200 got 401
Failed steps:
  Then the response status should be 200`)
	if !strings.Contains(out, "Feature: Login") || !strings.Contains(out, "Scenario:") {
		t.Fatalf("hints = %q", out)
	}
}

func TestFormatFeedback_BBDSection(t *testing.T) {
	v := quality.Verdict{
		Pass: false,
		Results: []quality.GateResult{{
			Name:   "acceptance",
			Kind:   quality.KindBDD,
			Level:  "acceptance",
			Specs:  []string{"features/"},
			Pass:   false,
			Output: "Feature: Checkout\nScenario: pay with card\nexpected true got false",
		}},
	}
	text := quality.FormatFeedback(v, 1, 3)
	if !strings.Contains(text, "Living specs: features/") || !strings.Contains(text, "Feature: Checkout") {
		t.Fatalf("feedback = %s", text)
	}
}
