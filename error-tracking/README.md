# Error Tracking Extension

The `error-tracking` extension is a first-party service-backed extension for
Move Big Rocks, the AI-native service operations platform.

It is a real first-party product extension, intended for self-hosted
production use on the shared base.

This directory is the canonical public bundle source for the free public
`error-tracking` first-party bundle published from the public first-party
extensions repo at `MoveBigRocks/extensions`.

## Where The Runtime Source Lives Today

The bundle source for `error-tracking` lives in this directory. The current
in-process runtime implementation is still split into the platform repo:

- observability domain and processing:
  [`MoveBigRocks/platform/internal/observability`](https://github.com/MoveBigRocks/platform/tree/main/internal/observability)
- SQL stores and persistence:
  [`MoveBigRocks/platform/internal/infrastructure/stores/sql`](https://github.com/MoveBigRocks/platform/tree/main/internal/infrastructure/stores/sql)
- extension service-target registration:
  [`MoveBigRocks/platform/internal/platform/extensionruntime`](https://github.com/MoveBigRocks/platform/tree/main/internal/platform/extensionruntime)

So there is not a hidden separate repo for the runtime code. It is currently
compiled with the core platform, while this repo remains the public source for
the extension bundle, schema, assets, release tags, and published OCI refs.

Current package scope:

- Sentry-compatible public ingest routes
- compatibility paths:
  - `/api/envelope`
  - `/api/:projectNumber/envelope`
  - `/1/envelope`
- owned PostgreSQL schema `ext_demandops_error_tracking`

Canonical schema migrations:

- `migrations/000001_init.up.sql`
- `migrations/000002_rls.up.sql`

Those files are the canonical schema history for
`ext_demandops_error_tracking`. Their applied versions are recorded in
`core_extension_runtime.schema_migration_history`, not in
`public.schema_migrations`.

Runtime targets used by the in-process service-target runtime:

- `error-tracking.ingest.envelope`
- `error-tracking.ingest.envelope.project`
- `error-tracking.runtime.health`

Distribution status:

- intended to ship as a free public signed first-party bundle
- intended public OCI ref:
  `ghcr.io/movebigrocks/mbr-ext-error-tracking:<version>`
- release tag pattern:
  `error-tracking-v<version>`

Install from source during development:

```bash
mbr extensions install ./error-tracking --workspace WORKSPACE_ID
```

Install from the published bundle ref:

```bash
mbr extensions install ghcr.io/movebigrocks/mbr-ext-error-tracking:v0.8.20 --workspace WORKSPACE_ID
```

Public signed bundle installs do not need a token. Keep `--license-token` for
controlled instance-bound bundle flows.
