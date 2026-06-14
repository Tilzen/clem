# Quality gates sample

Minimal clem.yaml showing deterministic quality gates on a Go project.

```bash
docker build -f samples/Dockerfile --build-arg SAMPLE=quality-go -t clem-quality .
```

With `quality.enabled: true`, the runner executes gates after each agent session
and feeds failures back into the next iteration via `CLAUDE.local.md`.

This sample demonstrates:

- `baseline_suite` — shared gates for the fleet (`build`, `unit`, `lint`)
- `agents: [lead]` — integration gate only for the lead agent
- `quality_suite: [+integration]` — inherit baseline + extra gate
- `quality_suite: [build, unit]` — explicit override for reviewer
