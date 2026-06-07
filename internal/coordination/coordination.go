package coordination

import "fmt"

// Backend describes a coordination platform clem can use for agent task boards.
// Three are supported today: Discord (default), Slack, and GitHub. Anything
// agent-facing that differs between platforms — MCP server name, channel ID
// format, alert POST URL — lives here.
type Backend struct {
	Name           string // "discord" | "slack" | "github"
	MCPName        string // the key used in .mcp.json
	MCPBinary      string // absolute path to the MCP server binary
	TokenEnvVar    string // env-var name the MCP server reads for auth
	AlertTemplate  string // curl template for the watchdog alert (see RenderAlert)
	TaskBoardNotes string // short paragraph injected into CLAUDE.shared.md
}

// AlertParams carries the per-config values that expand AlertTemplate.
type AlertParams struct {
	// Channel is a Discord channel ID, Slack channel ID, or GitHub issue number.
	Channel string
	// Repo is the GitHub owner/name — only used when Name == "github".
	Repo string
	// Message is the alert body (may be a bash variable like $safe_msg).
	Message string
}

// RenderAlert expands a backend's AlertTemplate with the given parameters.
// Discord and Slack use two placeholders (channel, message). GitHub uses three
// (repo, issue number, message).
func RenderAlert(b Backend, p AlertParams) string {
	switch b.Name {
	case "github":
		return fmt.Sprintf(b.AlertTemplate, p.Repo, p.Channel, p.Message)
	default:
		return fmt.Sprintf(b.AlertTemplate, p.Channel, p.Message)
	}
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
	default:
		return Backend{}, fmt.Errorf("unknown coordination backend %q (valid: discord, slack, github)", name)
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
	TokenEnvVar: "GITHUB_TOKEN",
	// Repo and issue number come from coordination.github_repo and channels.alerts.
	AlertTemplate: `curl -s -X POST "https://api.github.com/repos/%s/issues/%s/comments" \
        -H "Authorization: Bearer $GITHUB_TOKEN" -H "Content-Type: application/json" \
        -H "Accept: application/vnd.github+json" \
        -d "{\"body\":\"%s\"}" > /dev/null 2>&1`,
	TaskBoardNotes: `Task board lives in GitHub Issues on the configured repo (github_repo).
Each task = one open issue. Status is tracked with labels: clem:todo →
clem:in-progress → clem:done or clem:blocked. Claim by self-assigning
(gh issue edit N --add-assignee @me), then re-read the issue to confirm you
won the claim. Report status via issue comments. Link PRs with "Closes #N".`,
}
