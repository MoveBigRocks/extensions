# Move Big Rocks First-Party Extensions

This repo is the public home for production-ready first-party Move Big Rocks
extensions and their release catalog.

Examples and scaffolds belong in `MoveBigRocks/extension-sdk`.

It is licensed under BSL 1.1 with the same no-resale rule used across the
public Move Big Rocks code and extension surfaces. See `LICENSE`.

## Production Intent

These are real first-party extensions, intended for self-hosted production use
on the same bounded Move Big Rocks runtime:

- installable in the standard extension lifecycle
- versioned and published as signed GHCR bundles
- designed to be useful on their own, not just illustrative samples
- public enough to inspect and learn from if you want to build your own

## First-Party Catalog

### ATS

Applicant tracking with a careers site, candidate workflows, recruiting queues,
and bundled operator/agent affordances on the shared base.

- source: `ats/`
- install ref: `ghcr.io/movebigrocks/mbr-ext-ats:<version>`

### Error Tracking

Sentry-compatible ingest and issue workflows connected to the same cases,
queues, and operational context as the rest of the system.

- source: `error-tracking/`
- install ref: `ghcr.io/movebigrocks/mbr-ext-error-tracking:<version>`

### Web Analytics

Privacy-first analytics with public ingest, tracking assets, admin dashboards,
and extension-owned analytics state.

- source: `web-analytics/`
- install ref: `ghcr.io/movebigrocks/mbr-ext-web-analytics:<version>`

## Install The Current Bundle Set

The current free public first-party bundle set is:

- ATS
- error tracking
- web analytics

Install them by OCI ref:

```bash
mbr extensions install ghcr.io/movebigrocks/mbr-ext-ats:v1.0.0 --workspace WORKSPACE_ID
mbr extensions install ghcr.io/movebigrocks/mbr-ext-error-tracking:v1.0.0 --workspace WORKSPACE_ID
mbr extensions install ghcr.io/movebigrocks/mbr-ext-web-analytics:v1.0.0 --workspace WORKSPACE_ID
```

Or install directly from a checked-out source directory during development:

```bash
mbr extensions install ./ats --workspace WORKSPACE_ID
mbr extensions install ./error-tracking --workspace WORKSPACE_ID
mbr extensions install ./web-analytics --workspace WORKSPACE_ID
```

## Repo Structure Decision

- keep first-party production extensions out of the core repo
- keep them together in one public first-party extensions repo for now
- keep templates, examples, and scaffolds in `MoveBigRocks/extension-sdk`
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
`catalog/public-bundles.json`.

Operational note: after the first GHCR publication for each package, set the
package visibility to `Public` in GitHub Packages so the OCI refs are
anonymously pullable.

## Repo Rules

- keep first-party extensions installable from source checkout
- keep the public set non-privileged
- publish the free public first-party bundle set from this public repo
- keep examples and scaffolds in `MoveBigRocks/extension-sdk`, not here
- do not use this repo as the source of truth for privileged first-party packs

## Learn From These

This repo should also be good inspiration for teams building their own
extensions:

- each extension is a real bounded product slice
- each one has a manifest, assets, migrations, and release tags
- each one is installable through the same extension lifecycle customers use
- the public source is intentionally inspectable rather than hidden behind a
  marketplace
