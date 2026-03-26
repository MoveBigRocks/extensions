# Move Big Rocks First-Party Extensions

This repo is the public home for production-ready first-party Move Big Rocks
extensions and their release catalog.

These are the extensions you install when you want the shared Move Big Rocks
base to go deeper into a specific product area without adding another SaaS
tool, another billing relationship, or another place for operational context to
fragment.

Examples and scaffolds live in
[`MoveBigRocks/extension-sdk`](https://github.com/MoveBigRocks/extension-sdk).

It is licensed under BSL 1.1 with the same no-resale rule used across the
public Move Big Rocks code and extension surfaces. See [LICENSE](LICENSE).

## Production Intent

These are real first-party extensions, intended for self-hosted production use
on the same bounded Move Big Rocks runtime:

- installable in the standard extension lifecycle
- versioned and published as signed GHCR bundles
- designed to be useful on their own, not just illustrative samples
- public enough to inspect and learn from if you want to build your own

## Where The Source Lives

This repo is the public source of truth for the first-party extension bundle
layer:

- `manifest.json`
- extension-owned `assets/`
- extension-owned `migrations/`
- release tags and published GHCR bundle refs

For service-backed extensions, some runtime implementation still lives in
[`MoveBigRocks/platform`](https://github.com/MoveBigRocks/platform) today.
That split is real, and it is not the end state we want long-term.

Current runtime locations:

- `error-tracking` bundle source lives here; the in-process runtime currently
  lives in
  [`platform/internal/observability`](https://github.com/MoveBigRocks/platform/tree/main/internal/observability),
  [`platform/internal/infrastructure/stores/sql`](https://github.com/MoveBigRocks/platform/tree/main/internal/infrastructure/stores/sql),
  and
  [`platform/internal/platform/extensionruntime`](https://github.com/MoveBigRocks/platform/tree/main/internal/platform/extensionruntime)
- `web-analytics` bundle source lives here; the in-process runtime currently
  lives in
  [`platform/internal/analytics`](https://github.com/MoveBigRocks/platform/tree/main/internal/analytics),
  [`platform/internal/infrastructure/stores/sql`](https://github.com/MoveBigRocks/platform/tree/main/internal/infrastructure/stores/sql),
  and
  [`platform/internal/platform/extensionruntime`](https://github.com/MoveBigRocks/platform/tree/main/internal/platform/extensionruntime)
- `ats` is mostly declarative in this repo today, and builds on the shared
  platform primitives and generic extension runtime in
  [`MoveBigRocks/platform`](https://github.com/MoveBigRocks/platform)

## Why This Repo Exists

The point of this repo is simple:

- use ATS without paying for a separate recruiting SaaS too early
- run Sentry-compatible error tracking on infrastructure you control
- get privacy-first website analytics without bolting on another silo
- inspect real extension source if you want to build your own

These extensions are meant to be compelling in their own right, and also good
examples of how to build bounded products on the Move Big Rocks primitives.

## First-Party Catalog

### ATS

Applicant tracking for teams that want a serious careers page and structured
candidate handling without immediately paying for Home Run, Greenhouse, or a
similar dedicated ATS.

What it gives you:

- a branded careers site served from the extension
- a public application flow that creates candidate cases
- recruiting queues and workflow tags on the same operational base
- candidate evaluation built on Move Big Rocks cases, contacts, forms, queues,
  automation, and other shared primitives
- a natural path toward richer hiring flows, including CV or resume handling,
  because the underlying platform already has attachment-capable primitives
- a strong self-hosted foundation for teams that want to keep hiring context in
  the same system as the rest of their operations

Good fit:

- startups that want to save money early
- teams that want recruiting on the same base as support, operations, and
  knowledge
- teams that want to inspect or extend the full source instead of renting a
  black box

- source: [`ats/`](./ats)
- install ref: `ghcr.io/movebigrocks/mbr-ext-ats:<version>`

### Error Tracking

Sentry-compatible error tracking for teams that want application issues to live
on the same operational base as the rest of their work.

What it gives you:

- Sentry-compatible ingest endpoints
- issue workflows connected to shared queues, cases, and follow-up paths
- self-hosted control over error data and issue handling
- a first-party extension that can sit closer to support and operational
  response than a separate monitoring silo

Good fit:

- teams already using Sentry SDKs or Sentry-style envelopes
- teams that want to replace or reduce dependency on Sentry
- teams that want errors and operational response in one system

Compatibility note: Sentry-compatible ingest is the core positioning here.

- source: [`error-tracking/`](./error-tracking)
- install ref: `ghcr.io/movebigrocks/mbr-ext-error-tracking:<version>`

### Web Analytics

Privacy-first, cookie-free web analytics for teams that want useful traffic
visibility without dragging basic website reporting into another external tool.

What it gives you:

- a lightweight first-party tracking script
- public event ingest and workspace admin dashboards
- extension-owned analytics state on infrastructure you control
- a simple self-hosted analytics option that stays close to the rest of your
  operating context

Good fit:

- teams that want something simpler and more controllable than Google Analytics
- teams that do not want to pay for Plausible just to get privacy-first
  analytics
- teams that want website analytics on the same base as operations and support

Positioning note: cookie-free, privacy-first analytics out of the box.

- source: [`web-analytics/`](./web-analytics)
- install ref: `ghcr.io/movebigrocks/mbr-ext-web-analytics:<version>`

## Install The Current Bundle Set

The current free public first-party bundle set is:

- ATS
- error tracking
- web analytics

Install them by OCI ref:

```bash
mbr extensions install ghcr.io/movebigrocks/mbr-ext-ats:v0.8.21 --workspace WORKSPACE_ID
mbr extensions install ghcr.io/movebigrocks/mbr-ext-error-tracking:v0.8.20 --workspace WORKSPACE_ID
mbr extensions install ghcr.io/movebigrocks/mbr-ext-web-analytics:v0.8.20 --workspace WORKSPACE_ID
```

Or install directly from a checked-out source directory during development:

```bash
mbr extensions install ./ats --workspace WORKSPACE_ID
mbr extensions install ./error-tracking --workspace WORKSPACE_ID
mbr extensions install ./web-analytics --workspace WORKSPACE_ID
```

Then validate and activate the installed extension in the usual lifecycle:

```bash
mbr extensions validate --id EXTENSION_ID
mbr extensions activate --id EXTENSION_ID
```

## Repo Structure Decision

- keep first-party production extensions out of the core repo
- keep them together in one public first-party extensions repo for now
- keep templates, examples, and scaffolds in
  [`MoveBigRocks/extension-sdk`](https://github.com/MoveBigRocks/extension-sdk)
- split extensions into separate repos later only if ownership, release
  cadence, or compliance needs diverge

Use `--license-token` only for a controlled instance-bound bundle flow. The
free public signed bundle set installs without a token.

## Publication Model

This repo is the canonical public publication surface for the free public
first-party bundle set:

- `ghcr.io/movebigrocks/mbr-ext-ats:<version>`
- `ghcr.io/movebigrocks/mbr-ext-error-tracking:<version>`
- `ghcr.io/movebigrocks/mbr-ext-web-analytics:<version>`

Release tags are:

- `ats-v<version>`
- `error-tracking-v<version>`
- `web-analytics-v<version>`

The machine-readable catalog for the public bundle set lives in
[`catalog/public-bundles.json`](./catalog/public-bundles.json).

Packages are created by
[`public-bundles.yml`](./.github/workflows/public-bundles.yml) when one of the
release tags below is pushed. If the GitHub Packages tab is empty, the first
tagged publish has not completed yet.

Operational note: after the first GHCR publication for each package, set the
package visibility to `Public` in GitHub Packages so the OCI refs are
anonymously pullable.

The end-to-end publish and install runbook lives in
[`docs/PUBLISH_AND_INSTALL.md`](./docs/PUBLISH_AND_INSTALL.md).

## Repo Rules

- keep first-party extensions installable from source checkout
- keep the public set non-privileged
- publish the free public first-party bundle set from this public repo
- keep examples and scaffolds in
  [`MoveBigRocks/extension-sdk`](https://github.com/MoveBigRocks/extension-sdk),
  not here
- do not use this repo as the source of truth for privileged first-party packs

## Learn From These

This repo should also be good inspiration for teams building their own
extensions:

- each extension is a real bounded product slice
- each one has a manifest, assets, migrations, and release tags
- each one is installable through the same extension lifecycle customers use
- the public source is intentionally inspectable rather than hidden behind a
  marketplace
