# Community Feature Requests Extension

This extension is currently a first-party public beta. Install pinned version tags
like `ghcr.io/movebigrocks/mbr-ext-community-feature-requests:v0.1.0` while
the public board and admin workflow keep iterating.

This is the first-party `community-feature-requests` extension for Move Big Rocks.

It turns the public idea-board spec into a real installable extension with a runtime
shape that fits the platform today:

- extension-owned request and vote state in a dedicated Postgres schema
- public server-rendered idea board and detail pages
- anonymous voting with a bounded cookie-based voter key
- internal admin dashboard for status and visibility updates

## First Slice

The current implementation focuses on the main feedback loop:

- submit a new idea from the public board
- browse, search, and sort public ideas
- upvote a request once per browser/session key
- update status and visibility from the admin dashboard

Still intentionally left for later:

- public comments
- automatic linked-case creation and vote-threshold automations
- richer markdown rendering and roadmap/article linking
- deeper queue and knowledge integrations inside the admin view

## Source Layout

- [`manifest.json`](./manifest.json)
- [`extension.contract.json`](./extension.contract.json)
- [`migrations/`](./migrations)
- [`runtime/`](./runtime)
- [`runtimeui/`](./runtimeui)
- [`assets/agent-skills/`](./assets/agent-skills)

## Install

```bash
mbr extensions lint ./community-feature-requests --json
mbr extensions verify ./community-feature-requests --workspace WORKSPACE_ID --json
```

Runtime image tag pattern:

- `community-feature-requests-v<version>`
