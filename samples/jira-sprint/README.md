# clem sample — Jira Software sprint coordination

Uses **Jira Software** as the task board instead of Discord, Slack, or GitHub Issues.
Agents use **mcp-atlassian** (`jira-mcp`) to discover, claim, and report on tasks.
`clem provision` installs a per-agent **Jira watcher sidecar** that polls the REST API
and wakes the tmux session when new claimable issues appear.

## Prerequisites

- Jira Cloud site (Atlassian Cloud)
- `pipx install mcp-atlassian` on the host
- Project with labels: `clem-todo`, `clem-in-progress`, `clem-done`, `clem-blocked`
- Vault secrets per agent: `JIRA_USERNAME`, `JIRA_API_TOKEN`, and `GH_TOKEN` for PRs

## Setup

1. Create labels on your Jira project.
2. Choose alert/lessons strategy in `clem.yaml` (see configurable modes below).
3. Export env vars (or edit `clem.yaml`):

```bash
export JIRA_SITE=your-org.atlassian.net
export JIRA_PROJECT=ENG
export JIRA_ALERTS_ISSUE=OPS-12      # when alerts_mode: comment
export JIRA_LESSONS_ISSUE=OPS-34     # when lessons_mode: issue
```

4. Initialize agent docs:

```bash
clem init --backend jira
```

5. Store secrets:

```bash
clem vault init
clem vault set jira JIRA_USERNAME="you@company.com"
clem vault set jira JIRA_API_TOKEN="..."
clem vault set github GH_TOKEN="ghp_..."
sudo clem provision
sudo clem login
sudo clem up
```

## Configurable modes

| Field | Default | Options |
|-------|---------|---------|
| `jira.alerts_mode` | `comment` | `comment` posts to a fixed issue; `issue` creates a new incident ticket per alert |
| `jira.status_mode` | `labels` | `labels` only; `transitions` moves workflow columns; `both` |
| `jira.lessons_mode` | `issue` | `issue` comments on a meta-issue; `confluence` appends to a Confluence page |

Example — incident tickets instead of a shared alerts issue:

```yaml
coordination:
  jira:
    alerts_mode: issue
    alerts_label: clem-incident
    alerts_issue_type: Incident
```

## Task board convention

| Concept | Jira primitive |
|---------|----------------|
| Task | Issue with label `clem-todo` |
| Sprint scope | Optional `jira.jql_extra: "AND sprint in openSprints()"` |
| Status | Configurable via `jira.status_mode` (labels by default) |
| Claim | Assign yourself via jira-mcp, then re-read the issue |
| Updates | Comment on the issue |
| Output | PR in git + link issue key in comment |
| Alerts | `comment` on alerts issue, or `issue` creates incident tickets |
| Lessons | `issue` comments, or `confluence` page append |

## How it differs from other backends

| Aspect | Chat backends | GitHub backend | Jira backend |
|--------|---------------|----------------|--------------|
| Task discovery | MCP + watcher | `gh` CLI + issue watcher | jira-mcp + JQL watcher |
| Claim | Thread / emoji | Self-assign + re-read | Assign + re-read |
| Alerts | Channel post | Issue comment | Comment or new issue (configurable) |
| MCP | discord / slack | None | mcp-atlassian |
| Extra service | — | clem-github-watch | clem-jira-watch |

## Egress containment

When `egress:` is enabled, `{jira.site}` is automatically added to the egress allowlist
when `backend: jira`. The watcher uses the same loopback proxy as the agent service.

See also the [Jira coordination section](../../README.md#jira-coordination) in the main README.
