# Quality gates + living Gherkin specs (SDD/BDD)

This sample shows **Spec-Driven Development** with clem: Gherkin features in
`features/` are executable acceptance gates, not dead documentation.

After each agent session, clem runs `godog features/` and injects failing
scenarios into the next iteration via the closed feedback loop.

```yaml
# See clem.yaml — kind: bdd binds features/ to the acceptance gate
```

Wire your step definitions in the project repo as usual; clem only orchestrates
*when* specs run and *how* failures return to the agent.

See [docs/quality-gates.md](../../docs/quality-gates.md) for the full model.
