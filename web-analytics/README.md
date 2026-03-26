# Web Analytics Extension

The `web-analytics` extension is a first-party service-backed extension for Move Big Rocks,
the AI-native service operations platform.

It is a real first-party product extension, intended for self-hosted
production use on the shared base.

This directory is the canonical public bundle source for the free public
`web-analytics` first-party bundle published from the public first-party
extensions repo at `MoveBigRocks/extensions`.

Current package scope:

- public tracking script at `/js/analytics.js`
- public ingest endpoint at `/api/analytics/event`
- admin pages under `/admin/extensions/web-analytics/*`
- owned PostgreSQL schema `ext_demandops_web_analytics`

Canonical schema migrations:

- `migrations/000001_init.up.sql`
- `migrations/000002_rls.up.sql`

Those files are the canonical schema history for
`ext_demandops_web_analytics`. Their applied versions are recorded in
`core_extension_runtime.schema_migration_history`, not in
`public.schema_migrations`.

Runtime targets used by the in-process service-target runtime:

- `analytics.asset.script`
- `analytics.ingest.event`
- `analytics.admin.properties`
- `analytics.admin.property.dashboard`
- `analytics.admin.property.setup`
- `analytics.admin.property.settings`

Distribution status:

- intended to ship as a free public signed first-party bundle
- intended public OCI ref:
  `ghcr.io/movebigrocks/mbr-ext-web-analytics:<version>`
- release tag pattern:
  `web-analytics-v<version>`

Install from source during development:

```bash
mbr extensions install ./web-analytics --workspace WORKSPACE_ID
```

Install from the published bundle ref:

```bash
mbr extensions install ghcr.io/movebigrocks/mbr-ext-web-analytics:v1.0.0 --workspace WORKSPACE_ID
```

Public signed bundle installs do not need a token. Keep `--license-token` for
controlled instance-bound bundle flows.
