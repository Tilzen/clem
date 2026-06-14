package quality

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	FeedbackStartMarker = "<!-- clem:quality-feedback:start -->"
	FeedbackEndMarker   = "<!-- clem:quality-feedback:end -->"
	FeedbackMaxBytes    = 4096
)

// FormatFeedback renders a human-readable failure summary for agent consumption.
// BDD gates emphasize living Gherkin spec context so agents converge on the
// behaviors encoded in features/, not just generic test output.
func FormatFeedback(v Verdict, attempt, maxAttempts int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "[quality] Deterministic checks failed (attempt %d/%d). Use this feedback to fix the code — specs are the source of truth:\n\n", attempt, maxAttempts)
	for _, r := range v.Results {
		if r.Pass {
			continue
		}
		kind := KindOrDefault(r.Kind)
		fmt.Fprintf(&b, "### Gate: %s (%s, kind=%s)\n", r.Name, r.Level, kind)
		if len(r.Specs) > 0 {
			fmt.Fprintf(&b, "Living specs: %s\n", strings.Join(r.Specs, ", "))
		}
		fmt.Fprintf(&b, "Command failed (exit %d", r.ExitCode)
		if r.Error != "" {
			fmt.Fprintf(&b, ", %s", r.Error)
		}
		b.WriteString(")\n")
		if kind == KindBDD || r.Level == "acceptance" {
			if hints := ExtractBDDHints(r.Output); hints != "" {
				b.WriteString("Failing scenarios / steps:\n```\n")
				b.WriteString(truncateOutput(hints, 1500))
				b.WriteString("\n```\n")
			}
		}
		if r.Output != "" {
			b.WriteString("Output:\n```\n")
			b.WriteString(truncateOutput(r.Output, 1500))
			b.WriteString("\n```\n")
		}
		b.WriteString("\n")
	}
	return truncateOutput(b.String(), FeedbackMaxBytes)
}

// WriteFeedbackFile writes runner-visible feedback for the next prompt prepend.
func WriteFeedbackFile(homeDir, text string) error {
	if err := os.MkdirAll(filepath.Join(homeDir, ".clem"), 0700); err != nil {
		return err
	}
	if text == "" {
		return os.Remove(FeedbackPath(homeDir))
	}
	return os.WriteFile(FeedbackPath(homeDir), []byte(text), 0600)
}

// InjectFeedbackBlock replaces or inserts the delimited block in CLAUDE.local.md.
func InjectFeedbackBlock(claudeLocalPath, feedback string) error {
	var existing []byte
	if data, err := os.ReadFile(claudeLocalPath); err == nil {
		existing = data
	}
	content := replaceFeedbackBlock(string(existing), feedback)
	return os.WriteFile(claudeLocalPath, []byte(content), 0644)
}

func replaceFeedbackBlock(content, feedback string) string {
	start := strings.Index(content, FeedbackStartMarker)
	end := strings.Index(content, FeedbackEndMarker)
	if start >= 0 && end > start {
		end += len(FeedbackEndMarker)
		before := strings.TrimSpace(content[:start])
		after := strings.TrimSpace(content[end:])
		if feedback == "" {
			switch {
			case before == "" && after == "":
				return ""
			case before == "":
				return after
			case after == "":
				return before
			default:
				return before + "\n\n" + after
			}
		}
		block := buildFeedbackBlock(feedback)
		switch {
		case before == "" && after == "":
			return block
		case before == "":
			return block + "\n\n" + after
		case after == "":
			return before + "\n\n" + block
		default:
			return before + "\n\n" + block + "\n\n" + after
		}
	}
	content = strings.TrimSpace(content)
	if feedback == "" {
		return content
	}
	block := buildFeedbackBlock(feedback)
	if content == "" {
		return block
	}
	return content + "\n\n" + block
}

func buildFeedbackBlock(feedback string) string {
	return FeedbackStartMarker + "\n" + feedback + "\n" + FeedbackEndMarker
}

// ClearFeedback removes feedback from both file and CLAUDE.local.md.
func ClearFeedback(homeDir, claudeLocalPath string) error {
	_ = os.Remove(FeedbackPath(homeDir))
	if claudeLocalPath == "" {
		return nil
	}
	return InjectFeedbackBlock(claudeLocalPath, "")
}
