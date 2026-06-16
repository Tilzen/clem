package coordination

import (
	"strings"
	"testing"
)

func TestKnown_Backends(t *testing.T) {
	tests := []struct {
		name    string
		want    string
		wantErr bool
	}{
		{"", "discord", false},
		{"discord", "discord", false},
		{"slack", "slack", false},
		{"github", "github", false},
		{"jira", "jira", false},
		{"gitlab", "", true},
	}
	for _, tc := range tests {
		b, err := Known(tc.name)
		if tc.wantErr {
			if err == nil {
				t.Errorf("Known(%q) expected error", tc.name)
			}
			continue
		}
		if err != nil {
			t.Fatalf("Known(%q): %v", tc.name, err)
		}
		if b.Name != tc.want {
			t.Errorf("Known(%q).Name = %q, want %q", tc.name, b.Name, tc.want)
		}
	}
}

func TestRenderAlert_Discord(t *testing.T) {
	b, _ := Known("discord")
	got := RenderAlert(b, AlertParams{Channel: "123", Message: "hello"})
	want := `https://discord.com/api/v10/channels/123/messages`
	if !strings.Contains(got, want) {
		t.Fatalf("RenderAlert discord missing %q:\n%s", want, got)
	}
	if !strings.Contains(got, `hello`) {
		t.Fatalf("RenderAlert discord missing message:\n%s", got)
	}
}

func TestRenderAlert_Slack(t *testing.T) {
	b, _ := Known("slack")
	got := RenderAlert(b, AlertParams{Channel: "C123", Message: "ping"})
	if !strings.Contains(got, `slack.com/api/chat.postMessage`) {
		t.Fatalf("RenderAlert slack missing API URL:\n%s", got)
	}
	if !strings.Contains(got, `\"channel\":\"C123\"`) {
		t.Fatalf("RenderAlert slack missing channel:\n%s", got)
	}
}

func TestRenderAlert_GitHub(t *testing.T) {
	b, _ := Known("github")
	got := RenderAlert(b, AlertParams{
		Repo:    "owner/repo",
		Channel: "42",
		Message: "alert body",
	})
	for _, want := range []string{
		`api.github.com/repos/owner/repo/issues/42/comments`,
		`Authorization: Bearer $GH_TOKEN`,
		`alert body`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("RenderAlert github missing %q:\n%s", want, got)
		}
	}
}

func TestGitHub_TokenEnvVar(t *testing.T) {
	b, _ := Known("github")
	if b.TokenEnvVar != "GH_TOKEN" {
		t.Fatalf("github TokenEnvVar = %q, want GH_TOKEN", b.TokenEnvVar)
	}
}

func TestAlertCurlGuard_GitHubSkipsWhenAlertsUnset(t *testing.T) {
	b, _ := Known("github")
	got := AlertCurlGuard(b, "", `curl example`, "")
	if got != "true" {
		t.Fatalf("expected no-op when alerts unset, got %q", got)
	}
}

func TestAlertCurlGuard_GitHubRequiresTokenAndIssue(t *testing.T) {
	b, _ := Known("github")
	got := AlertCurlGuard(b, "42", `curl example`, "")
	for _, want := range []string{`[ -n "$GH_TOKEN" ]`, `[ -n "42" ]`, `curl example`} {
		if !strings.Contains(got, want) {
			t.Fatalf("AlertCurlGuard missing %q:\n%s", want, got)
		}
	}
}

func TestRenderAlert_Jira(t *testing.T) {
	b, _ := Known("jira")
	got := RenderAlert(b, AlertParams{
		Repo:    "acme.atlassian.net",
		Channel: "OPS-12",
		Message: "alert body",
	})
	for _, want := range []string{
		`https://acme.atlassian.net/rest/api/3/issue/OPS-12/comment`,
		`JIRA_USERNAME:$JIRA_API_TOKEN`,
		`alert body`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("RenderAlert jira missing %q:\n%s", want, got)
		}
	}
}

func TestRenderAlert_JiraIssueMode(t *testing.T) {
	b, _ := Known("jira")
	got := RenderAlert(b, AlertParams{
		Repo:            "acme.atlassian.net",
		Message:         "disk full",
		JiraProject:     "ENG",
		JiraAlertsMode:  "issue",
		JiraAlertsLabel: "clem-incident",
		JiraIssueType:   "Incident",
	})
	for _, want := range []string{
		`https://acme.atlassian.net/rest/api/3/issue`,
		`project`,
		`ENG`,
		`disk full`,
		`Incident`,
		`clem-incident`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("RenderAlert jira issue mode missing %q:\n%s", want, got)
		}
	}
}

func TestAlertCurlGuard_JiraIssueModeSkipsIssueKey(t *testing.T) {
	b, _ := Known("jira")
	got := AlertCurlGuard(b, "", `curl example`, "issue")
	for _, want := range []string{
		`[ -n "$JIRA_API_TOKEN" ]`,
		`[ -n "$JIRA_USERNAME" ]`,
		`curl example`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("AlertCurlGuard jira issue mode missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, `[ -n "" ]`) {
		t.Fatalf("issue mode should not require channels.alerts:\n%s", got)
	}
}

func TestAlertCurlGuard_JiraSkipsWhenAlertsUnset(t *testing.T) {
	b, _ := Known("jira")
	got := AlertCurlGuard(b, "", `curl example`, "comment")
	if got != "true" {
		t.Fatalf("expected no-op when alerts unset, got %q", got)
	}
}

func TestAlertCurlGuard_JiraRequiresTokenUserAndIssue(t *testing.T) {
	b, _ := Known("jira")
	got := AlertCurlGuard(b, "OPS-1", `curl example`, "comment")
	for _, want := range []string{
		`[ -n "$JIRA_API_TOKEN" ]`,
		`[ -n "$JIRA_USERNAME" ]`,
		`[ -n "OPS-1" ]`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("AlertCurlGuard jira missing %q:\n%s", want, got)
		}
	}
}
