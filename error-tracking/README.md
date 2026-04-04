# Error Tracking Extension

The `error-tracking` extension is a first-party service-backed extension for
Move Big Rocks, the AI-native service operations platform.

It is a real first-party product extension, intended for self-hosted
production use on the shared base.

This directory is the canonical public bundle source for the free public
`error-tracking` first-party bundle published from the public first-party
extensions repo at `MoveBigRocks/extensions`.

## Runtime Source

The public runtime source for `error-tracking` is in this directory:

- contract assertions:
  [`extension.contract.json`](./extension.contract.json)
- runtime domain, handlers, resolvers, services, and SQL-backed store code:
  [`runtime/`](./runtime)
- admin templates:
  [`templates/`](./templates)
- SQL model definitions used by the runtime store:
  [`sql-models/`](./sql-models)

This directory is the runtime source that people should inspect and learn from.
Platform-owned host contracts it depends on are exposed through public packages,
not through core repo internals.

Package scope:

- Sentry-compatible public ingest routes
- public ingest paths:
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
mbr extensions lint ./error-tracking --json
mbr extensions verify ./error-tracking --workspace WORKSPACE_ID --json
```

Install from the published bundle ref:

```bash
mbr extensions install ghcr.io/movebigrocks/mbr-ext-error-tracking:v<VERSION> --workspace WORKSPACE_ID
```

Public signed bundle installs do not need a token. Keep `--license-token` for
controlled instance-bound bundle flows.
