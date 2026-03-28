# Sales Pipeline Extension

This pack is currently a first-party public beta. Install pinned version tags
like `ghcr.io/movebigrocks/mbr-ext-sales-pipeline:v0.1.0` rather than relying
on floating tags while the surface continues to tighten.

This is the first-party `sales-pipeline` extension for Move Big Rocks.

It turns the current sales spec into an installable service-backed pack that
fits the platform as it exists today:

- extension-owned deal and stage state in a dedicated Postgres schema
- shared-primitives hooks through seeded forms, queues, and automation rules
- an admin board for reviewing, creating, and moving opportunities
- a B2B/B2C mode switch driven by extension config rather than a brand-new core
  record type

## First Slice

The current implementation intentionally lands the smallest useful version:

- stage-based opportunity board with totals by stage
- quick create for new deals
- stage movement through the board UI
- default stage seeding on first use
- seeded intake form and queue for shared-primitives workflow handoff

Still intentionally left for later:

- deep contact and organization linking beyond captured identifiers
- activity timelines and attachments
- reporting beyond board totals
- stage-change automation driven from extension-owned events
- richer configuration UI for stage editing

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
