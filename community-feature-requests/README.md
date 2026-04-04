# Community Feature Requests Extension

This is the first-party public beta `community-feature-requests` extension for
Move Big Rocks. Install pinned version tags such as
`ghcr.io/movebigrocks/mbr-ext-community-feature-requests:v0.1.0`.

It provides:

- extension-owned request and vote state in a dedicated Postgres schema
- public server-rendered idea board and detail pages
- anonymous voting with a bounded cookie-based voter key
- internal admin dashboard for status and visibility updates

- submit a new idea from the public board
- browse, search, and sort public ideas
- upvote a request once per browser/session key
- update status and visibility from the admin dashboard

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
