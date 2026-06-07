# GitHub coordination sample

Uses **GitHub Issues** as the task board instead of Discord or Slack. Agents use the
`gh` CLI directly (no GitHub MCP) to discover, claim, and report on tasks.

## Setup

1. Create a task repository (or use an existing one) and add labels:
   - `clem:todo`, `clem:in-progress`, `clem:done`, `clem:blocked`
2. Open two meta-issues for alerts and lessons; note their issue numbers.
3. Export env vars (or edit `clem.yaml` directly):

```bash
export GITHUB_TASKS_REPO=your-org/your-tasks
export GITHUB_ALERTS_ISSUE=12
export GITHUB_LESSONS_ISSUE=34
```

4. Initialize agent docs:

```bash
clem init --backend github
```

5. Provision as usual (`clem vault init`, `clem provision`).

## How it differs from Discord/Slack samples

| Aspect | Chat backends | GitHub backend |
|--------|---------------|----------------|
| Task discovery | MCP + gateway watcher (Discord) | Sidecar polls Issues + `gh issue list` fallback |
| Claim | Thread prefix / emoji | Self-assign + re-read issue |
| Alerts | Post to #alerts channel | Comment on alerts issue |
| MCP | discord-bot / slack-mcp | None (gh CLI only) |

See [PRD-github-coordinator.md](../../PRD-github-coordinator.md) for the full design.
