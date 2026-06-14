# Quality gates — closed feedback loop

clem quality gates turn **deterministic checks** into a **closed feedback loop**
for agent fleets. You declare what “correct” means for your project; after each
agent session the runner executes those checks and feeds structured failures back
into the next iteration — before broken work reaches CI or a PR.

`quality.enabled: false` (default) preserves today’s behaviour exactly.

---

## Why quality gates?

Agent fleets are good at producing code quickly, but non-deterministic LLM output
needs an objective arbiter. Quality gates provide that arbiter **locally and
iteratively**:

| Benefit | What it means in practice |
|---------|---------------------------|
| **Faster convergence** | Agents fix failing tests/lint/BDD scenarios in the next iteration instead of waiting for CI |
| **Lower CI noise** | Fewer red builds and review cycles on obvious failures |
| **Token discipline** | Feedback replaces a delimited block in `CLAUDE.local.md` each iteration — it does not accumulate |
| **Fleet observability** | Per-agent JSONL metrics, `clem quality`, and the QUALITY column in `clem status` |
| **Extensible checks** | Shell commands today; Gherkin/BDD (`kind: bdd`) for living acceptance specs |
| **Zero regression default** | Disabled unless explicitly enabled in `clem.yaml` |

Gates are a **local pre-filter**. GitHub/GitLab CI remains the merge authority.

---

## End-to-end workflow

Each agent in the fleet runs inside `clem-runner.sh`. When quality gates are
enabled and provisioned, every session follows this loop:

```
┌─────────────────────────────────────────────────────────────────┐
│  1. START ITERATION                                             │
│     Runner loads prior feedback from ~/.clem/quality-feedback.txt│
│     → prepended to the agent prompt as RUNNER_WARNINGS          │
│     → injected block in CLAUDE.local.md (delimited markers)     │
└────────────────────────────┬────────────────────────────────────┘
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│  2. AGENT SESSION                                               │
│     Claude Code / opencode works on the task in $WORKDIR        │
└────────────────────────────┬────────────────────────────────────┘
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│  3. POST-SESSION GATES  (clem quality run)                      │
│     Read ~/.clem/quality.json (provisioned per agent)           │
│     Run agent's gate suite in order (fail-fast on blocking gates)│
│     Append results to ~/.clem/<agent>-quality.jsonl             │
└────────────────────────────┬────────────────────────────────────┘
                             ▼
                    ┌────────┴────────┐
                    │  All gates pass? │
                    └────────┬────────┘
                  yes │               │ no
                      ▼               ▼
              Clear feedback    Format verdict
              Reset attempts    Write quality-feedback.txt
              exit 0            Inject CLAUDE.local.md block
                                Increment attempt counter
                                exit 1 (retry next iteration)
                                      │
                                      ▼
                                max_attempts reached?
                                      │
                              yes ──► mark task BLOCKED
                                      alert coordination channel
                                      exit 2
```

### What the agent sees on failure

Feedback is structured and gate-specific. For command gates, stdout/stderr
(truncated) is included. For BDD gates, Feature/Scenario lines are extracted so
Gherkin remains the source of truth:

```
[quality] Quality gates failed (attempt 2/3)

Gate: unit (level: unit)
Exit: 1
Output:
--- FAIL: TestHandler (0.01s)
    handler_test.go:42: expected 200 got 500
```

On pass, the feedback file is removed and the delimited block is stripped from
`CLAUDE.local.md` entirely.

---

## Fleet integration

Quality gates are **per-agent**, provisioned at `clem provision` time:

| Artifact | Path | Purpose |
|----------|------|---------|
| Runtime config | `~/.clem/quality.json` | Gate suite, on_failure mode, max_attempts |
| Feedback | `~/.clem/quality-feedback.txt` | Runner reads before next session |
| State | `~/.clem/quality-state.json` | Attempt counter, blocked flag, task ID |
| Metrics | `~/.clem/<agent>-quality.jsonl` | One JSON line per gate result |
| Task binding | `~/.clem/current-task-id` | Resets attempts when task changes |

The runner hook (in `~/bin/clem-runner.sh`) runs automatically — no agent
cooperation required:

```bash
# After each session:
clem quality run --home "$HOME" --workdir "$WORKDIR"

# Before each session:
# feedback loaded from ~/.clem/quality-feedback.txt into prompt
```

Pre-push (`clem quality pre-push`) runs only when `on_failure: block-push` is
configured, as Pass 5 of the git pre-push hook installed by `clem provision`.

---

## Gate layers: general + agent-specific

Gates can be shared across the fleet or scoped to individual agents.

| Layer | Where defined | Who runs it (default suite) |
|-------|---------------|----------------------------|
| **Baseline** | `quality.baseline_suite` or gates without `agents:` | Every agent |
| **Agent-restricted** | `quality.gates[].agents: [lead]` | Only listed agents |
| **Inline** | `agents.<key>.quality_gates` | Only that agent |

### Default suite (no `quality_suite`)

When an agent has no `quality_suite`, clem resolves:

1. **Baseline** — gates in `baseline_suite`, or (if unset) all global gates with no `agents:` restriction, sorted by level
2. **Agent-restricted global gates** — gates whose `agents:` includes this agent
3. **Inline gates** — `quality_gates` on the agent

Agent-restricted gates in the global catalog are **not** run by other agents.

### Explicit suite

```yaml
quality_suite: [build, unit]
```

Runs **only** those gates, in that order. Each gate must exist and be allowed
for the agent.

### Inherit + extras

Extend the default suite:

```yaml
quality_suite: [+lint]              # baseline + lint
quality_suite: [inherit, +acceptance]
```

---

## Configuration reference

### Top-level `quality` block

```yaml
quality:
  enabled: true                      # default: false
  on_failure: feedback               # feedback | block-push | advisory
  max_attempts: 3                    # closed-loop retries before BLOCKED
  baseline_suite: [build, unit, lint]
  gates:
    - name: build
      kind: command                  # default; explicit shell command
      level: build
      cmd: "go build ./..."
      blocking: true                 # fail-fast; stops suite on failure
      timeout: 120s                  # default 5m
      retries: 0                     # re-run gate N times before failing
    - name: acceptance
      kind: bdd                      # living Gherkin specs
      level: acceptance
      specs: [features/]
      runner: godog                  # godog | behave | cucumber
      agents: [lead]                 # only lead runs by default
      blocking: true
      timeout: 600s
```

### `on_failure` modes

| Mode | Iteration behaviour | Pre-push |
|------|---------------------|----------|
| `feedback` | Fail → write feedback → retry until pass or max_attempts → BLOCKED | No gate check |
| `block-push` | Gates run but iteration is not blocked; metrics recorded | Blocking gates must pass to push |
| `advisory` | Gates run; failures logged to JSONL only; no feedback loop | No gate check |

### Per-agent overrides

```yaml
agents:
  lead:
    quality_suite: [+acceptance]     # default + extra gate
    quality_gates:                   # agent-local gate definitions
      - name: lead-smoke
        level: custom
        cmd: "./scripts/lead-smoke.sh"
        blocking: true

  reviewer:
    quality_suite: [build, unit]     # explicit subset; no lint, no acceptance
```

### Gate kinds

| `kind` | Use case |
|--------|----------|
| `command` | Any deterministic check: `go test`, `eslint`, `terraform validate`, custom scripts |
| `bdd` | **Living Gherkin specs** — `features/*.feature` as executable acceptance tests |

For BDD/SDD, stack cheap checks first:

1. `build` / `typecheck` — compile sanity
2. `unit` — fast regression
3. `bdd` (`level: acceptance`) — behaviour from Gherkin via godog/behave/cucumber

Override with an explicit `cmd` anytime you need a custom runner invocation.

### Gate levels (canonical order)

`build` → `typecheck` → `unit` → `integration` → `lint` → `acceptance` → `custom`

When no explicit order is given, gates are sorted by level.

---

## Observability

```bash
# Per-agent summary from JSONL metrics
clem quality

# Fleet status including QUALITY column
clem status
```

Example `clem quality` output:

```
AGENT      LAST     GATES        ATTEMPT    PASS RATE
------------------------------------------------------------
lead       PASS     3/3          0          85%
reviewer   FAIL     1/2          2          60%
```

---

## Getting started

1. Add a `quality:` block to `clem.yaml` (see `samples/quality-go/` or
   `samples/quality-bdd/`)
2. Run `clem provision` — writes `~/.clem/quality.json` and updates runner hooks
   for each agent
3. Agents converge locally; CI validates before merge

### Samples

| Sample | Demonstrates |
|--------|--------------|
| [`samples/quality-go/`](../samples/quality-go/) | Command gates, baseline suite, agent-restricted integration gate |
| [`samples/quality-bdd/`](../samples/quality-bdd/) | Gherkin acceptance gates with `kind: bdd` |

---

## Design notes

- **Deterministic only** — gates are shell commands with exit-code semantics (0 = pass). No LLM-as-judge.
- **Fail-fast** — blocking gate failure stops the suite; non-blocking failures are recorded but do not fail the run.
- **Task-scoped attempts** — changing `~/.clem/current-task-id` resets the attempt counter.
- **Timeout exit 124** — gates exceeding their timeout are reported as timeout failures.
- **Provision required** — editing `clem.yaml` alone does not update running agents; re-provision to refresh runtime config.
