package coordination

import (
	"encoding/json"
	"fmt"
	"strings"
)

// MCPAtlassianVersion is the pinned PyPI release for mcp-atlassian install instructions.
const MCPAtlassianVersion = "0.21.1"

// MCPAtlassianInstallCmd returns the pinned pipx install command for mcp-atlassian.
func MCPAtlassianInstallCmd() string {
	return "pipx install mcp-atlassian==" + MCPAtlassianVersion
}

// Backend describes a coordination platform clem can use for agent task boards.
// Four are supported today: Discord (default), Slack, GitHub, and Jira. Anything
// agent-facing that differs between platforms — MCP server name, channel ID
// format, alert POST URL — lives here.
type Backend struct {
	Name           string // "discord" | "slack" | "github" | "jira"
	MCPName        string // the key used in .mcp.json
	TokenEnvVar    string // env-var name the MCP server reads for auth
	AlertTemplate  string // curl template for the watchdog alert (see RenderAlert)
	TaskBoardNotes string // short paragraph injected into CLAUDE.shared.md
}

// AlertParams carries the per-config values that expand AlertTemplate.
type AlertParams struct {
	// Channel is a Discord channel ID, Slack channel ID, GitHub issue number,
	// or Jira issue key (e.g. OPS-12). Unused for Jira when AlertsMode is issue.
	Channel string
	// Repo is the GitHub owner/name when Name == "github", or the Jira site
	// hostname (e.g. acme.atlassian.net) when Name == "jira".
	Repo string
	// Message is the alert body (may be a bash variable like $safe_msg).
	Message string
	// JiraProject is the Jira project key when Name == "jira".
	JiraProject string
	// JiraAlertsMode is comment or issue (Jira only).
	JiraAlertsMode string
	// JiraAlertsLabel labels new issues when JiraAlertsMode is issue.
	JiraAlertsLabel string
	// JiraIssueType is the issue type name for created alert issues.
	JiraIssueType string
}

// RenderAlert expands a backend's AlertTemplate with the given parameters.
// Discord and Slack use two placeholders (channel, message). GitHub uses three
// (repo, issue number, message).
func RenderAlert(b Backend, p AlertParams) string {
	switch b.Name {
	case "jira":
		if p.JiraAlertsMode == "issue" {
			return renderJiraIssueAlert(p)
		}
		return renderJiraCommentAlert(p.Repo, p.Channel, p.Message)
	case "github":
		return fmt.Sprintf(b.AlertTemplate, p.Repo, p.Channel, p.Message)
	default:
		return fmt.Sprintf(b.AlertTemplate, p.Channel, p.Message)
	}
}

func renderJiraCommentAlert(site, issueKey, message string) string {
	if message == "$safe_msg" {
		return fmt.Sprintf(jiraCommentAlertRuntime, site, issueKey)
	}
	payload, err := json.Marshal(jiraADFBody(message))
	if err != nil {
		return ""
	}
	return fmt.Sprintf(`curl -s -X POST "https://%s/rest/api/3/issue/%s/comment" \
        -u "$JIRA_USERNAME:$JIRA_API_TOKEN" -H "Content-Type: application/json" \
        -H "Accept: application/json" \
        -d "%s" > /dev/null 2>&1`, site, issueKey, escapeForBashDoubleQuoted(string(payload)))
}

func renderJiraIssueAlert(p AlertParams) string {
	if p.Message == "$safe_msg" {
		return fmt.Sprintf(jiraIssueAlertRuntime, p.Repo, p.JiraProject, p.JiraIssueType, p.JiraAlertsLabel)
	}
	payload, err := json.Marshal(map[string]any{
		"fields": map[string]any{
			"project":   map[string]any{"key": p.JiraProject},
			"summary":   p.Message,
			"issuetype": map[string]any{"name": p.JiraIssueType},
			"labels":    []string{p.JiraAlertsLabel},
		},
	})
	if err != nil {
		return ""
	}
	return fmt.Sprintf(`curl -s -X POST "https://%s/rest/api/3/issue" \
        -u "$JIRA_USERNAME:$JIRA_API_TOKEN" -H "Content-Type: application/json" \
        -H "Accept: application/json" \
        -d "%s" > /dev/null 2>&1`, p.Repo, escapeForBashDoubleQuoted(string(payload)))
}

func jiraADFBody(text string) map[string]any {
	return map[string]any{
		"body": map[string]any{
			"type":    "doc",
			"version": 1,
			"content": []any{
				map[string]any{
					"type": "paragraph",
					"content": []any{
						map[string]any{"type": "text", "text": text},
					},
				},
			},
		},
	}
}

// escapeForBashDoubleQuoted escapes a string embedded in a bash double-quoted
// curl -d argument (JSON payload assembled in Go at codegen time).
func escapeForBashDoubleQuoted(s string) string {
	return strings.NewReplacer(
		`\`, `\\`,
		`"`, `\"`,
		`$`, `\$`,
		"`", "\\`",
	).Replace(s)
}

const jiraCommentAlertRuntime = `curl -s -X POST "https://%s/rest/api/3/issue/%s/comment" \
        -u "$JIRA_USERNAME:$JIRA_API_TOKEN" -H "Content-Type: application/json" \
        -H "Accept: application/json" \
        -d "$(python3 -c "import json,sys; print(json.dumps({'body':{'type':'doc','version':1,'content':[{'type':'paragraph','content':[{'type':'text','text':sys.argv[1]}]}]}}))" "$msg")" > /dev/null 2>&1`

const jiraIssueAlertRuntime = `curl -s -X POST "https://%s/rest/api/3/issue" \
        -u "$JIRA_USERNAME:$JIRA_API_TOKEN" -H "Content-Type: application/json" \
        -H "Accept: application/json" \
        -d "$(python3 -c "import json,sys; t,pk,itn,lb=sys.argv[1:5]; print(json.dumps({'fields':{'project':{'key':pk},'summary':t,'issuetype':{'name':itn},'labels':[lb]}}))" "$msg" "%s" "%s" "%s")" > /dev/null 2>&1`

// AlertCurlGuard wraps an alert curl body with token (and GitHub issue) guards.
// Skips the curl when backend is github and channels.alerts is unset.
// For Jira, jiraAlertsMode may be "issue" (create per alert) or "comment"
// (default; requires a channels.alerts issue key).
func AlertCurlGuard(b Backend, channel, body, jiraAlertsMode string) string {
	if b.Name == "github" && strings.TrimSpace(channel) == "" {
		return "true"
	}
	if b.Name == "jira" {
		if jiraAlertsMode == "issue" {
			return `[ -n "$JIRA_API_TOKEN" ] && [ -n "$JIRA_USERNAME" ] && ` + body
		}
		if strings.TrimSpace(channel) == "" {
			return "true"
		}
	}
	guard := fmt.Sprintf(`[ -n "$%s" ]`, b.TokenEnvVar)
	if b.Name == "github" || b.Name == "jira" {
		guard += fmt.Sprintf(` && [ -n "%s" ]`, strings.TrimSpace(channel))
	}
	if b.Name == "jira" {
		guard += ` && [ -n "$JIRA_USERNAME" ]`
	}
	return guard + ` && ` + body
}

// Known returns the backend for a config value. Unknown backends return an
// error that surfaces at config load time.
func Known(name string) (Backend, error) {
	switch name {
	case "", "discord":
		return discord, nil
	case "slack":
		return slack, nil
	case "github":
		return github, nil
	case "jira":
		return jira, nil
	default:
		return Backend{}, fmt.Errorf("unknown coordination backend %q (valid: discord, slack, github, jira)", name)
	}
}

var discord = Backend{
	Name:        "discord",
	MCPName:     "discord-bot",
	TokenEnvVar: "DISCORD_TOKEN",
	// Raw bot token (no "Bot " prefix) — clem strips it on vault set.
	AlertTemplate: `curl -s -X POST "https://discord.com/api/v10/channels/%s/messages" \
        -H "Authorization: Bot $DISCORD_TOKEN" -H "Content-Type: application/json" \
        -d "{\"content\":\"%s\"}" > /dev/null 2>&1`,
	TaskBoardNotes: `Task board lives in #tasks (forum channel). Each task = one thread.
Use list_threads (not read_messages) to discover work. Status lives in the
thread's first-message prefix: [TODO] → [IN PROGRESS] → [DONE] or [BLOCKED].`,
}

var slack = Backend{
	Name:        "slack",
	MCPName:     "slack-mcp",
	TokenEnvVar: "SLACK_MCP_XOXP_TOKEN",
	AlertTemplate: `curl -s -X POST "https://slack.com/api/chat.postMessage" \
        -H "Authorization: Bearer $SLACK_MCP_XOXP_TOKEN" -H "Content-Type: application/json; charset=utf-8" \
        -d "{\"channel\":\"%s\",\"text\":\"%s\"}" > /dev/null 2>&1`,
	TaskBoardNotes: `Task board lives in #tasks (regular channel). Each task = the top-level
message; updates happen inside its thread. Status lives as a reaction emoji on
the top message: ⏳ (TODO) → 🔨 (IN PROGRESS) → ✅ (DONE) or ⛔ (BLOCKED).
Slack has no forum channel type — threads replace first-class forum posts.`,
}

var github = Backend{
	Name:        "github",
	TokenEnvVar: "GH_TOKEN",
	// Repo and issue number come from coordination.github_repo and channels.alerts.
	AlertTemplate: `curl -s -X POST "https://api.github.com/repos/%s/issues/%s/comments" \
        -H "Authorization: Bearer $GH_TOKEN" -H "Content-Type: application/json" \
        -H "Accept: application/vnd.github+json" \
        -d "{\"body\":\"%s\"}" > /dev/null 2>&1`,
	TaskBoardNotes: `Task board lives in GitHub Issues on the configured repo (github_repo).
Each task = one open issue. Status is tracked with labels: clem:todo →
clem:in-progress → clem:done or clem:blocked. Claim by self-assigning
(gh issue edit N --add-assignee @me), then re-read the issue to confirm you
won the claim. Report status via issue comments. Link PRs with "Closes #N".`,
}

var jira = Backend{
	Name:        "jira",
	MCPName:     "jira-mcp",
	TokenEnvVar: "JIRA_API_TOKEN",
	// RenderAlert builds Jira alert curls (ADF comment or issue create).
	AlertTemplate: ``,
	TaskBoardNotes: `Task board lives in the configured Jira project (jira.project). Each task =
one issue with the label from channels.tasks (e.g. clem-todo). Status tracking
is configurable via jira.status_mode (labels, transitions, or both). Discover
work with jira_search (JQL) or jira-mcp; claim by assigning yourself, then
re-read the issue. Alerts: jira.alerts_mode comment (issue key) or issue
(per-incident tickets). Lessons: jira.lessons_mode issue or confluence.`,
}
