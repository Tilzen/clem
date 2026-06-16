package config

import "fmt"

// JiraCoordination holds Jira-specific coordination settings when backend is jira.
type JiraCoordination struct {
	// Site is the Atlassian Cloud hostname (e.g. acme.atlassian.net), no scheme.
	Site string `yaml:"site"`
	// Project is the Jira project key (e.g. ENG).
	Project string `yaml:"project"`
	// JQLExtra is an optional fragment appended to the task-board JQL in the
	// issue watcher, e.g. `AND sprint in openSprints()` for sprint-scoped work.
	JQLExtra string `yaml:"jql_extra"`
	// AlertsMode controls how watchdog/runtime alerts are delivered.
	// "comment" (default): POST a comment on channels.alerts issue key.
	// "issue": create a new Jira issue per alert (like human incident filing).
	AlertsMode string `yaml:"alerts_mode"`
	// AlertsLabel is applied to issues created when alerts_mode is issue.
	AlertsLabel string `yaml:"alerts_label"`
	// AlertsIssueType is the Jira issue type name for created alerts (default Task).
	AlertsIssueType string `yaml:"alerts_issue_type"`
	// StatusMode controls how agents express workflow progress.
	// "labels" (default): swap clem-* labels only — board columns are untouched.
	// "transitions": move workflow status via jira_transition_issue.
	// "both": labels and workflow transitions.
	StatusMode string `yaml:"status_mode"`
	// LessonsMode controls where post-mortems are recorded.
	// "issue" (default): comment on channels.lessons issue key.
	// "confluence": append to a Confluence page via mcp-atlassian.
	LessonsMode string `yaml:"lessons_mode"`
	// LessonsPageID is the Confluence page ID when lessons_mode is confluence.
	LessonsPageID string `yaml:"lessons_page_id"`
}

func (j JiraCoordination) AlertsModeOrDefault() string {
	if j.AlertsMode == "" {
		return "comment"
	}
	return j.AlertsMode
}

func (j JiraCoordination) AlertsLabelOrDefault() string {
	if j.AlertsLabel == "" {
		return "clem-incident"
	}
	return j.AlertsLabel
}

func (j JiraCoordination) AlertsIssueTypeOrDefault() string {
	if j.AlertsIssueType == "" {
		return "Task"
	}
	return j.AlertsIssueType
}

func (j JiraCoordination) StatusModeOrDefault() string {
	if j.StatusMode == "" {
		return "labels"
	}
	return j.StatusMode
}

func (j JiraCoordination) LessonsModeOrDefault() string {
	if j.LessonsMode == "" {
		return "issue"
	}
	return j.LessonsMode
}

// AlertProtocolDoc returns agent-facing markdown for the configured alerts mode.
func (j JiraCoordination) AlertProtocolDoc(alertsIssueKey string) string {
	switch j.AlertsModeOrDefault() {
	case "issue":
		return fmt.Sprintf(
			"Create a new issue in project **%s** with label `%s` (jira_create_issue via jira-mcp). Each watchdog or runtime alert becomes its own ticket — the usual Jira incident pattern.",
			j.Project, j.AlertsLabelOrDefault(),
		)
	default:
		if alertsIssueKey != "" {
			return fmt.Sprintf("Comment on issue **%s** (jira_add_comment).", alertsIssueKey)
		}
		return "Comment on the configured alerts issue key (jira_add_comment)."
	}
}

// StatusProtocolDoc returns agent-facing markdown for the configured status mode.
func (j JiraCoordination) StatusProtocolDoc() string {
	switch j.StatusModeOrDefault() {
	case "transitions":
		return "Move issues across workflow columns with **jira_transition_issue** (e.g. To Do → In Progress → Done). Labels are optional."
	case "both":
		return "Swap clem-* labels **and** transition workflow status (jira_transition_issue) when starting, completing, or blocking work."
	default:
		return "Swap labels only: clem-todo → clem-in-progress → clem-done / clem-blocked. Board columns are **not** moved automatically — set `jira.status_mode: transitions` or `both` if your team tracks status via workflow columns."
	}
}

// LessonsProtocolDoc returns agent-facing markdown for the configured lessons mode.
func (j JiraCoordination) LessonsProtocolDoc(lessonsIssueKey string) string {
	switch j.LessonsModeOrDefault() {
	case "confluence":
		if j.LessonsPageID != "" {
			return fmt.Sprintf(
				"Append lessons to Confluence page **%s** via confluence MCP tools (mcp-atlassian supports Jira + Confluence). Format: Problem → Root cause → Solution → Prevention.",
				j.LessonsPageID,
			)
		}
		return "Append lessons to the configured Confluence page via confluence MCP tools. Format: Problem → Root cause → Solution → Prevention."
	default:
		if lessonsIssueKey != "" {
			return fmt.Sprintf(
				"Read and post lessons as comments on issue **%s**. Format: Problem → Root cause → Solution → Prevention.",
				lessonsIssueKey,
			)
		}
		return "Read and post lessons as comments on the configured lessons issue. Format: Problem → Root cause → Solution → Prevention."
	}
}
