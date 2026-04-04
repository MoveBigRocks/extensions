# Move Big Rocks First-Party Extensions

This repo is the home for production-ready first-party Move Big Rocks
extensions and the release catalog for the public OCI bundle set.

These are the extensions you install when you want the shared Move Big Rocks
base to go deeper into a specific product area without adding another SaaS
tool, another billing relationship, or another place for operational context to
fragment.

Examples and scaffolds live in
[`MoveBigRocks/extension-sdk`](https://github.com/MoveBigRocks/extension-sdk).

It is licensed under the MBR Source Code Available License 1.0. Teams may use
and modify it for their own company, use the first-party extensions, and build
their own extensions, but may not sell the platform, extensions, derivative
works, or hosted access without separate written permission. See
[LICENSE](LICENSE).

## Production Intent

These are real first-party extensions, intended for self-hosted production use
on the same bounded Move Big Rocks runtime:

- installable in the standard extension lifecycle
- versioned and installable as signed extension bundles
- designed to be useful on their own, not just illustrative samples
- public enough to inspect and learn from if you want to build your own

## Where The Source Lives

This repo is the source of truth for the first-party extension source:

- `manifest.json`
- extension-owned `assets/`
- extension-owned `migrations/`
- runtime implementation under each extension directory
- templates and admin UI artifacts
- release tags and published GHCR bundle refs

Source layout:

- every first-party extension ships an `extension.contract.json` file that
  captures its expected menu, routes, seeded resources, commands, and skills
- `ats/` contains the ATS bundle, skills, templates, and ATS-specific domain source
- `ats/runtime/domain/` defines Go concepts like vacancies, vacancy catalogs,
  applicants, and applications
- `enterprise-access/` contains the Enterprise Access manifest, admin skill,
  and owned-schema migrations
- `web-analytics/runtime/` contains the web analytics runtime source
- `web-analytics/templates/` contains the analytics admin templates
- `error-tracking/runtime/` contains the error tracking runtime source
- `error-tracking/templates/` contains the error tracking admin templates
- `error-tracking/sql-models/` contains the SQL model definitions used by the
  error tracking runtime
- `sales-pipeline/runtime/` contains the sales board runtime and deal storage
- `community-feature-requests/runtime/` contains the public idea-board runtime

The service-backed runtime source that first-party and external authors should
inspect is in this repo, with shared host-facing contracts exposed
through the public SDK instead of borrowed from the core repo.

First-party extension code in this repo should depend only on public extension
surfaces:

- `MoveBigRocks/extension-sdk` for runtime helpers like `runtimehttp` and
  `extdb`
- `github.com/movebigrocks/extension-sdk/extensionhost/...` for host-facing
  public types and contracts

It should not import anything from `github.com/movebigrocks/platform/...`.

## Why This Repo Exists

The point of this repo is simple:

- use ATS without paying for a separate recruiting SaaS too early
- run enterprise SSO from the same first-party extension base
- run Sentry-compatible error tracking on infrastructure you control
- get privacy-first website analytics without bolting on another silo
- keep lightweight sales flow inside the same operating base
- collect community roadmap feedback without another voting SaaS
- inspect real extension source if you want to build your own

These extensions are meant to be compelling in their own right, and also good
examples of how to build bounded products on the Move Big Rocks primitives.

## Validation Standard

Every first-party extension in this repo is expected to pass the same
contract-first lifecycle we want custom extension authors to use:

```bash
mbr extensions lint ./EXTENSION_DIR --json
mbr extensions verify ./EXTENSION_DIR --workspace WORKSPACE_ID --json
mbr extensions nav --instance --json
mbr extensions widgets --instance --json
```

If an extension intentionally changes its declared extension surface, refresh its
contract file and review the diff:

```bash
mbr extensions lint ./EXTENSION_DIR --write-contract --json
```

For workspace-scoped admin UI, passing validation also means the extension stays
discoverable for an instance admin with no active workspace selection. The
instance-level menu entry should still open a working page, and static admin
pages that call workspace-bound APIs should preserve the `?workspace=...` hint.

To re-run the strict first-party catalog proof locally, use:

```bash
./scripts/check-public-boundary.sh
MBR_BIN=/path/to/mbr bash ./scripts/validate-first-party.sh
bash ./scripts/report-first-party-release-state.sh
```

The release-state report shows whether each extension manifest version already
has a matching git release tag locally and on `origin`. Versioned public bundle
publishes should come from matching git tags, not ad hoc workflow-dispatch
semver strings.

## First-Party Catalog

### ATS

Applicant tracking for teams that want a serious careers page and structured
candidate handling without immediately paying for Home Run, Greenhouse, or a
similar dedicated ATS.

What it gives you:

- a branded careers site served from the extension
- a public job and general-application flow that creates candidate cases
- ATS-owned vacancy lifecycle and application-stage logic in Go
- ATS-owned branding, content, and managed careers media for the public site
- saved views, stage presets, and a guided setup checklist in the ATS admin
- recruiting queues and workflow tags on the same operational base
- candidate evaluation built on Move Big Rocks cases, contacts, queues,
  attachments, automation, and other shared primitives
- resume or CV linkage through the shared attachment primitives already present
  in the platform
- talent-pool routing on top of shared case and queue primitives
- a strong self-hosted foundation for teams that want to keep hiring context in
  the same system as the rest of their operations

Good fit:

- startups that want to save money early
- teams that want recruiting on the same base as support, operations, and
  knowledge
- teams that want to inspect or extend the full source instead of renting a
  black box

- source: [`ats/`](./ats)
- install ref: `ghcr.io/movebigrocks/mbr-ext-ats:v<version>`

### Enterprise Access

Enterprise SSO for teams that want Move Big Rocks to authenticate against an
existing IdP without breaking the shared extension model.

What it gives you:

- OIDC-first provider configuration
- instance-scoped extension install and activation
- admin settings and provider persistence in the extension-owned schema
- OIDC start and callback flows backed by the same runtime model
- runtime health and a guided setup skill for operators and agents

Good fit:

- teams that want first-party SSO on the same extension base
- operators who want identity setup to stay inspectable and versioned
- teams that want Enterprise Access source in the same repo as the other
  first-party extensions

- source: [`enterprise-access/`](./enterprise-access)
- install from source during development: `mbr extensions install ./enterprise-access`

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
- install ref: `ghcr.io/movebigrocks/mbr-ext-error-tracking:v<version>`

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
- install ref: `ghcr.io/movebigrocks/mbr-ext-web-analytics:v<version>`

### Sales Pipeline (Public Beta)

Lightweight CRM and pipeline tracking for teams that want a practical
opportunity board inside Move Big Rocks instead of adding a separate deal tool
too early.

What it gives you:

- extension-owned deal and stage state in a dedicated schema
- workspace-native board UI for reviewing and moving opportunities
- seeded intake form, queue, and follow-up automation hook
- a B2B/B2C mode switch driven from extension config

Good fit:

- teams that want a simple opportunity board without paying for a full CRM
- operators who already use Move Big Rocks for intake and follow-up
- builders who want a service-backed example of product state plus shared
  primitives

- source: [`sales-pipeline/`](./sales-pipeline)
- install ref: `ghcr.io/movebigrocks/mbr-ext-sales-pipeline:v<version>`
- install guidance: pin an explicit version tag

### Community Feature Requests (Public Beta)

Public idea collection and voting for teams that want customer feedback and
internal triage on the same base.

What it gives you:

- public idea board and detail pages
- anonymous one-vote-per-browser upvoting
- admin dashboard for status and visibility changes
- extension-owned request state that stays close to internal workflow

- source: [`community-feature-requests/`](./community-feature-requests)
- install ref: `ghcr.io/movebigrocks/mbr-ext-community-feature-requests:v<version>`
- install guidance: pin an explicit version tag

Good fit:

- product teams that want a self-hosted feedback board
- teams that want public roadmap conversation without another SaaS
- builders who want a service-backed public-page extension example

## Install The Public OCI Set

The free public first-party bundle set is:

- ATS
- community feature requests
- error tracking
- sales pipeline
- web analytics

Install them by OCI ref:

```bash
mbr extensions install ghcr.io/movebigrocks/mbr-ext-ats:v<VERSION> --workspace WORKSPACE_ID
mbr extensions install ghcr.io/movebigrocks/mbr-ext-community-feature-requests:v<VERSION> --workspace WORKSPACE_ID
mbr extensions install ghcr.io/movebigrocks/mbr-ext-error-tracking:v<VERSION> --workspace WORKSPACE_ID
mbr extensions install ghcr.io/movebigrocks/mbr-ext-sales-pipeline:v<VERSION> --workspace WORKSPACE_ID
mbr extensions install ghcr.io/movebigrocks/mbr-ext-web-analytics:v<VERSION> --workspace WORKSPACE_ID
```

Or install directly from a checked-out source directory during development:

```bash
mbr extensions install ./ats --workspace WORKSPACE_ID
mbr extensions install ./community-feature-requests --workspace WORKSPACE_ID
mbr extensions install ./enterprise-access
mbr extensions install ./error-tracking --workspace WORKSPACE_ID
mbr extensions install ./sales-pipeline --workspace WORKSPACE_ID
mbr extensions install ./web-analytics --workspace WORKSPACE_ID
```

Then validate and activate the installed extension in the usual lifecycle:

```bash
mbr extensions validate --id EXTENSION_ID
mbr extensions activate --id EXTENSION_ID
```

## Repo Structure Decision

- keep first-party production extensions out of the core repo
- keep them together in one first-party extensions repo
- keep templates, examples, and scaffolds in
  [`MoveBigRocks/extension-sdk`](https://github.com/MoveBigRocks/extension-sdk)
- split extensions into separate repos later only if ownership, release
  cadence, or compliance needs diverge

Use `--license-token` only for a controlled instance-bound bundle flow. The
free public signed bundle set installs without a token.

## Publication Model

This repo is the canonical source for every first-party extension.

The public OCI publication surface in this repo is:

- `ghcr.io/movebigrocks/mbr-ext-ats:v<version>`
- `ghcr.io/movebigrocks/mbr-ext-community-feature-requests:v<version>`
- `ghcr.io/movebigrocks/mbr-ext-error-tracking:v<version>`
- `ghcr.io/movebigrocks/mbr-ext-sales-pipeline:v<version>`
- `ghcr.io/movebigrocks/mbr-ext-web-analytics:v<version>`

Release tags are:

- `ats-v<version>`
- `community-feature-requests-v<version>`
- `error-tracking-v<version>`
- `sales-pipeline-v<version>`
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
- keep every first-party extension source here
- publish the free public first-party bundle set from this public repo
- keep examples and scaffolds in
  [`MoveBigRocks/extension-sdk`](https://github.com/MoveBigRocks/extension-sdk),
  not here

## Learn From These

This repo should also be good inspiration for teams building their own
extensions:

- each extension is a real bounded product slice
- each one has a manifest, assets, migrations, and release tags
- each one is installable through the same extension lifecycle customers use
- the public source is intentionally inspectable rather than hidden behind a
  marketplace
