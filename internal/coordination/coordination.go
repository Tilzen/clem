package coordination

import (
	"fmt"
	"strings"
)

// Backend describes a coordination platform clem can use for agent task boards.
// Four are supported today: Discord (default), Slack, GitHub, and Jira. Anything
// agent-facing that differs between platforms — MCP server name, channel ID
// format, alert POST URL — lives here.
type Backend struct {
	Name           string // "discord" | "slack" | "github" | "jira"
	MCPName        string // the key used in .mcp.json
	MCPBinary      string // absolute path to the MCP server binary
	TokenEnvVar    string // env-var name the MCP server reads for auth
	AlertTemplate  string // curl template for the watchdog alert (see RenderAlert)
	TaskBoardNotes string // short paragraph injected into CLAUDE.shared.md
}

// AlertParams carries the per-config values that expand AlertTemplate.
type AlertParams struct {
	// Channel is a Discord channel ID, Slack channel ID, GitHub issue number,
	// or Jira issue key (e.g. OPS-12).
	Channel string
	// Repo is the GitHub owner/name when Name == "github", or the Jira site
	// hostname (e.g. acme.atlassian.net) when Name == "jira".
	Repo string
	// Message is the alert body (may be a bash variable like $safe_msg).
	Message string
}

// RenderAlert expands a backend's AlertTemplate with the given parameters.
// Discord and Slack use two placeholders (channel, message). GitHub uses three
// (repo, issue number, message).
func RenderAlert(b Backend, p AlertParams) string {
	switch b.Name {
	case "github", "jira":
		return fmt.Sprintf(b.AlertTemplate, p.Repo, p.Channel, p.Message)
	default:
		return fmt.Sprintf(b.AlertTemplate, p.Channel, p.Message)
	}
}

// AlertCurlGuard wraps an alert curl body with token (and GitHub issue) guards.
// Skips the curl when backend is github and channels.alerts is unset.
func AlertCurlGuard(b Backend, channel, body string) string {
	if b.Name == "github" && strings.TrimSpace(channel) == "" {
		return "true"
	}
	if b.Name == "jira" && strings.TrimSpace(channel) == "" {
		return "true"
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
	MCPBinary:   "/usr/local/bin/mcp-discord",
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
	MCPBinary:   "/usr/local/bin/slack-mcp-server",
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
	MCPBinary:   "/usr/local/bin/mcp-atlassian",
	TokenEnvVar: "JIRA_API_TOKEN",
	// Site hostname and issue key come from coordination.jira.site and channels.alerts.
	AlertTemplate: `curl -s -X POST "https://%s/rest/api/3/issue/%s/comment" \
        -u "$JIRA_USERNAME:$JIRA_API_TOKEN" -H "Content-Type: application/json" \
        -H "Accept: application/json" \
        -d "{\"body\":{\"type\":\"doc\",\"version\":1,\"content\":[{\"type\":\"paragraph\",\"content\":[{\"type\":\"text\",\"text\":\"%s\"}]}]}}" > /dev/null 2>&1`,
	TaskBoardNotes: `Task board lives in the configured Jira project (jira.project). Each task =
one issue with the label from channels.tasks (e.g. clem-todo). Status via labels:
clem-todo → clem-in-progress → clem-done or clem-blocked. Discover work with
jira_search (JQL) or the jira-mcp tools; claim by assigning yourself, then
re-read the issue to confirm you won the claim. Report via issue comments.`,
}
