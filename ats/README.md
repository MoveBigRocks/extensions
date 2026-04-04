# ATS Extension

The first-party `ats` extension turns a Move Big Rocks workspace into a
recruiting product with a generated careers site, public application intake,
and recruiter workflow built on shared MBR primitives.

## What It Does

- provisions a dedicated `Hiring` workspace on install
- stores recruiting truth in the ATS-owned PostgreSQL schema
- generates a branded public careers site at `/careers`
- generates public job pages at `/careers/jobs/:slug`
- supports a first-class general-application flow
- accepts ATS-native public applications and opaque resume-upload receipts
- creates one shared contact and one shared case for each ATS application
- routes candidate work across shared queues, attachments, tags, and automation
- lets recruiters manage stages, notes, saved views, stage presets, and
  talent-pool routing from the ATS admin surface at `/extensions/ats`

## Product Model

ATS owns:

- jobs
- applicants
- applications
- recruiter notes
- careers-site profile, team, gallery, media, and setup state
- saved views
- stage presets

Move Big Rocks shared primitives are reused deliberately:

- `contacts` for candidate identity
- `cases` for operational recruiting work
- `queues` for job routing, general applications, and talent pool
- `attachments` for resumes and uploaded careers media backing
- `tags` and automation for recruiting workflow follow-up

Submission-specific data is stored on the ATS `applications` records, so
multiple applications from the same person do not overwrite each other.
The runtime receives workspace context from the host and uses narrowed host
capabilities for shared primitives instead of reaching through broad core
stores inside the ATS business layer.

## Public Surface

- `/careers`
- `/careers/jobs/:slug`
- `/careers/applications`
- `/careers/attachments`

The public site is generated from ATS SQL data only. Open jobs render as live
job pages. Draft jobs are not rendered as public pages, and previously
published closed or paused jobs are replaced with an unavailable page.
Public submit and upload endpoints return public-safe receipts rather than
internal workspace, case, contact, or raw attachment identifiers.

## Admin Surface

The ATS builder at `/extensions/ats` includes:

- setup checklist with truthful completion state
- site profile and branding
- managed media uploads
- team and gallery editors
- jobs and job publishing controls
- candidate inbox with stage changes, notes, routing, and bulk actions
- saved views
- stage presets

## Source Layout

- manifest: [`manifest.json`](./manifest.json)
- contract: [`extension.contract.json`](./extension.contract.json)
- migrations: [`migrations/`](./migrations)
- admin and careers templates: [`assets/templates/`](./assets/templates)
- agent skills: [`assets/agent-skills/`](./assets/agent-skills)
- runtime domain: [`runtime/domain/`](./runtime/domain)
- runtime implementation: [`runtime/`](./runtime)

## Verification

Install from source during development:

```bash
mbr extensions lint ./ats --json
mbr extensions verify ./ats --workspace WORKSPACE_ID --json
mbr extensions install ./ats --workspace WORKSPACE_ID
```

Run the ATS runtime tests locally:

```bash
go test ./ats/runtime/... -count=1
jq empty ./ats/manifest.json ./ats/extension.contract.json
```

## Additional Docs

- target careers-site model:
  [`docs/careers-site-target.md`](./docs/careers-site-target.md)
- product requirements and roadmap:
  [`docs/ats-v1-requirements-and-roadmap.md`](./docs/ats-v1-requirements-and-roadmap.md)
- implementation plan:
  [`docs/ats-gap-closure-implementation-plan.md`](./docs/ats-gap-closure-implementation-plan.md)
