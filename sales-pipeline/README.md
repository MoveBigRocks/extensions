# Sales Pipeline Extension

This is the first-party public beta `sales-pipeline` extension for Move Big Rocks.
Install pinned version tags such as
`ghcr.io/movebigrocks/mbr-ext-sales-pipeline:v0.1.0`.

It provides:

- extension-owned deal and stage state in a dedicated Postgres schema
- shared-primitives hooks through seeded forms, queues, and automation rules
- an admin board for reviewing, creating, and moving opportunities
- a B2B/B2C mode switch driven by extension config rather than a brand-new core
  record type

- stage-based opportunity board with totals by stage
- quick create for new deals
- stage movement through the board UI
- default stage seeding on first use
- seeded intake form and queue for shared-primitives workflow handoff

## Source Layout

- [`manifest.json`](./manifest.json)
- [`extension.contract.json`](./extension.contract.json)
- [`migrations/`](./migrations)
- [`runtime/`](./runtime)
- [`runtimeui/`](./runtimeui)
- [`assets/agent-skills/`](./assets/agent-skills)

## Install

```bash
mbr extensions lint ./sales-pipeline --json
mbr extensions verify ./sales-pipeline --workspace WORKSPACE_ID --json
```

Runtime image tag pattern:

- `sales-pipeline-v<version>`
