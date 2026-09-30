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
  [`runtimeui/templates/`](./runtimeui/templates)
- SQL model definitions used by the runtime store:
  [`sql-models/`](./sql-models)

This directory is the runtime source that people should inspect and learn from.
It consumes the public `extension-sdk` and the host wire API. It does not
import platform Go packages or access core stores directly.

Package scope:

- Sentry-compatible public ingest routes
- public ingest paths:
  - `/api/envelope`
  - `/api/:projectNumber/envelope`
  - `/1/envelope`
- owned PostgreSQL schema `ext_demandops_error_tracking`

Workspace-scoped issue reads use `GET /extensions/error-tracking/api/agent/issues`
and `GET /extensions/error-tracking/api/agent/issues/:id` with an agent token,
the token's workspace in `?workspace=`, and `issue:read`. Session equivalents omit
`/agent`. The versioned response contract is `error-tracking/v1`; the host checks
current authority and the runtime always applies a workspace predicate.

Lists accept `project`, `status`, `level`, `limit` (1–100) and `offset` and return
`issues.nextOffset`. They are live views ordered by last seen and ID; deduplicate
IDs if new events arrive between pages. Detail returns the latest event's release,
environment and bounded exception/stack evidence. It excludes request headers,
users, local variables, breadcrumbs and arbitrary context. Error messages remain
untrusted application content. These reads do not change issue or case state.

Installation is instance-scoped; projects and issue reads retain workspace
isolation. A successful SDK flush or ingest HTTP response confirms acceptance,
not completed asynchronous issue projection. For a deployment canary, send a
labelled event with the actual application's environment/release settings, then
poll the scoped issue detail with a deadline and verify the exact event ID,
environment and release. Missing readback fails the canary.

Canonical schema migrations:

- `migrations/000001_init.up.sql`
- `migrations/000002_rls.up.sql`
- `migrations/000003_issue_case_claims.up.sql`

Those files are the canonical schema history for
`ext_demandops_error_tracking`. Their applied versions are recorded in
`core_extension_runtime.schema_migration_history`, not in
`public.schema_migrations`.

Selected service targets dispatched to the supervised child process over
`unix_socket_http` (see `manifest.json` for the full endpoint catalog):

- `error-tracking.ingest.envelope`
- `error-tracking.ingest.envelope.project`
- `error-tracking.runtime.health`

Distribution status:

- published as a free public signed first-party bundle through the
  [publication workflow](../.github/workflows/public-bundles.yml)
- public OCI ref:
  `ghcr.io/movebigrocks/mbr-ext-error-tracking:v<version>`
- release tag pattern:
  `error-tracking-v<version>`

Install from source during development:

```bash
mbr extensions lint ./error-tracking --json
mbr extensions verify ./error-tracking --workspace WORKSPACE_ID --json
```

Install from the published bundle ref:

```bash
mbr extensions install ghcr.io/movebigrocks/mbr-ext-error-tracking:v<VERSION>
```

Public signed bundle installs do not need a token. Keep `--license-token` for
controlled instance-bound bundle flows.
