package config

import "github.com/jahwag/clem/internal/coordination"

// CoordinationAlertParams builds alert curl parameters for the configured backend.
func (c *Config) CoordinationAlertParams(message string) coordination.AlertParams {
	p := coordination.AlertParams{
		Channel: c.Coordination.Channels["alerts"],
		Message: message,
	}
	switch c.Coordination.BackendOrDefault() {
	case "github":
		p.Repo = c.Coordination.GithubRepo
	case "jira":
		j := c.Coordination.Jira
		p.Repo = j.Site
		p.JiraProject = j.Project
		p.JiraAlertsMode = j.AlertsModeOrDefault()
		p.JiraAlertsLabel = j.AlertsLabelOrDefault()
		p.JiraIssueType = j.AlertsIssueTypeOrDefault()
	}
	return p
}

// JiraAlertsMode returns the effective Jira alerts mode (comment or issue).
func (c *Config) JiraAlertsMode() string {
	if !c.UsesJiraCoordination() {
		return ""
	}
	return c.Coordination.Jira.AlertsModeOrDefault()
}
