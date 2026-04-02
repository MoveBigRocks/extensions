# ATS Extension

This is the public first-party `ats` extension for Move Big Rocks, the
AI-native service operations platform.

It is positioned as a real product slice on the shared base, not as a toy
sample or throwaway example.

It is intentionally built from the same primitives customers will use:

- workspace-scoped installation
- ATS-owned PostgreSQL schema for recruiting workflow state
- job lifecycle modeled in Go
- ATS-owned careers-site profile, team, and gallery data
- ATS-owned managed careers media assets published into the website artifact surface
- seeded queues
- candidate contacts and cases created on the shared core services
- seeded `case_created` automation rule for ATS follow-up tagging
- seeded ATS stage-change automation rule for recruiter follow-up
- generated public careers site assets under `/careers`
- branded careers builder admin surface under `/extensions/ats`
- ATS-specific Go domain and runtime types for jobs, applicants, applications, setup state, notes, saved filters, and stage presets
- declared endpoints, namespaced extension commands, bundled agent-skill assets, extension events, and a service-backed runtime binary

This extension is intended to be part of the free public first-party bundle
set and is the canonical public bundle source for ATS.

## Source Layout

ATS owns a clear product slice today. Its public source lives in this
directory:

- bundle manifest:
  [`manifest.json`](./manifest.json)
- contract assertions:
  [`extension.contract.json`](./extension.contract.json)
- owned-schema migrations:
  [`migrations/`](./migrations)
- careers and admin templates:
  [`assets/templates/`](./assets/templates)
- agent skills:
  [`assets/agent-skills/`](./assets/agent-skills)
- ATS domain model source:
  [`runtime/domain/`](./runtime/domain)
- ATS service-backed runtime:
  [`runtime/`](./runtime)
- ATS runtime entry point:
  [`cmd/ats-runtime`](../cmd/ats-runtime)
- ATS scenario proof tool:
  [`tools/ats-scenario-proof`](../tools/ats-scenario-proof)

It still builds on shared platform primitives like cases, contacts, queues,
attachments, and automation, but the ATS-specific recruiting state now lives in
the extension-owned schema and runtime source here.

The current ATS Go model defines explicit ATS concepts that do not exist in the
shared base by default:

- `Vacancy` with lifecycle methods like publish, pause, close, and reopen
- `Vacancy` public-content sections for careers-site generation, including
  structured responsibilities, profile, offers, and quote fields
- `Vacancy.Kind` so ATS can distinguish publicly listed jobs from hidden system
  intake like general applications
- `CareersSiteProfile`, `CareersTeamMember`, and `CareersGalleryItem` for the
  company-level public site content and branding
- `CareersMediaAsset` for managed uploaded branding and careers imagery
- `CareersSetupState` for guided ATS onboarding progress
- `Applicant` with resume attachment linkage and profile/contact fields
- `Application` with explicit stages like `received`, `screening`,
  `interview`, `offer`, `hired`, `rejected`, and `withdrawn`
- `VacancyCatalog` for published-role lookup and application intake against the
  active vacancy set
- `CandidateSubmissionFromFields` to translate compatibility payloads into ATS
  applicant/application input when ATS is acting as an adapter

That means ATS no longer only relies on generic forms/cases language in public
source; the extension now carries its own product vocabulary in Go as well.

See [`docs/careers-site-target.md`](./docs/careers-site-target.md) for the
target careers-site model and the gap analysis against the inspiration site in
the sibling `careers/` folder.

See [`docs/ats-v1-requirements-and-roadmap.md`](./docs/ats-v1-requirements-and-roadmap.md)
for the detailed product requirements and delivery plan needed to make ATS
coherent, complete, and competitive.

See [`docs/ats-gap-closure-implementation-plan.md`](./docs/ats-gap-closure-implementation-plan.md)
for the detailed implementation sequencing, migrations, workstreams, and
milestones needed to close the gap between the current extension and the target
state. Those docs now include explicit status notes so the implemented ATS
baseline and the remaining parity roadmap are separated cleanly.

## Defensible Scope

The current ATS scope is intentionally real and bounded:

- define vacancies with open, paused, closed, and archived lifecycle states
- generate and publish a branded careers site from ATS-owned structured data
- manage careers-site brand media through ATS-owned uploaded assets and public
  artifact-backed URLs
- intake applications into Move Big Rocks using the ATS-native public endpoint
  plus contacts, cases, attachments, queues, tags, and automation
- track applicants and applications with ATS-owned fields and explicit stages
- upload and link resumes and CVs through the shared attachment primitives
- support both role-specific applications and a first-class general-application
  intake path
- let recruiters route candidate cases between the owning job queue and the
  shared talent pool while keeping ATS and MBR state aligned
- give operators and agents first-party skills for publishing roles and
  reviewing candidates

Things ATS still does not claim:

- a dedicated interview scheduling system
- scorecards or structured interview kits
- reporting and analytics parity with mature recruiting suites
- a dedicated offer approval workflow

Install it with the operator CLI by pointing at the directory directly:

```bash
mbr extensions lint ./ats --json
mbr extensions verify ./ats --workspace WORKSPACE_ID --json
mbr extensions install ./ats \
  --workspace WORKSPACE_ID
```

Or install the latest published public bundle ref. The newest published ATS tag
is currently `v0.8.25`; this branch is versioned for the next ATS release cut
and should be installed from source until that OCI tag is published:

```bash
mbr extensions install ghcr.io/movebigrocks/mbr-ext-ats:v0.8.25 \
  --workspace WORKSPACE_ID
```

Build and run the runtime binary locally against a platform database:

```bash
go test ./ats/runtime -count=1
go run ./tools/ats-scenario-proof --out /tmp/ats-proof.json
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ./cmd/ats-runtime
```

The CLI reads `manifest.json` plus every file under `assets/` and uploads the
bundle through the same extension install path used for first-party signed
public bundles and private custom bundles.

Public signed bundle installs do not need a token. Keep `--license-token` for
controlled instance-bound bundle flows.

Release tag pattern:

- `ats-v<version>`

Inspect the extension-declared agent skills with the generic CLI:

```bash
mbr extensions show --id EXTENSION_ID
mbr extensions skills list --id EXTENSION_ID
mbr extensions skills show --id EXTENSION_ID --name publish-job
```
