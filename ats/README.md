# ATS Extension

This is the public first-party `ats` extension for Move Big Rocks, the
AI-native service operations platform.

It is positioned as a real product slice on the shared base, not as a toy
sample or throwaway example.

It is intentionally built from the same primitives customers will use:

- workspace-scoped installation
- ATS-owned PostgreSQL schema for recruiting workflow state
- vacancy lifecycle modeled in Go
- seeded queues
- candidate contacts and cases created on the shared core services
- seeded `case_created` automation rule for ATS follow-up tagging
- seeded ATS stage-change automation rule for recruiter follow-up
- branded public/admin assets
- ATS-specific Go domain and runtime types for vacancies, applicants, applications, notes, saved filters, and stage presets
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
- `Applicant` with resume attachment linkage and profile/contact fields
- `Application` with explicit stages like `received`, `screening`,
  `interview`, `offer`, `hired`, `rejected`, and `withdrawn`
- `VacancyCatalog` for published-role lookup and application intake against the
  active vacancy set
- `CandidateSubmissionFromFields` to translate generic form payloads into ATS
  applicant/application input

That means ATS no longer only relies on generic forms/cases language in public
source; the extension now carries its own product vocabulary in Go as well.

## Defensible Scope

The current ATS scope is intentionally real and bounded:

- define vacancies with open, paused, closed, and archived lifecycle states
- publish a branded careers site from the extension bundle
- intake applications into Move Big Rocks using forms, contacts, cases, and
  ATS tags
- track applicants and applications with ATS-owned fields and explicit stages
- link resumes and CVs through the shared attachment primitives by attachment
  ID
- give operators and agents first-party skills for publishing roles and
  reviewing candidates

Things ATS still does not claim:

- a dedicated interview scheduling system
- a dedicated offer approval workflow

Install it with the operator CLI by pointing at the directory directly:

```bash
mbr extensions lint ./ats --json
mbr extensions verify ./ats --workspace WORKSPACE_ID --json
mbr extensions install ./ats \
  --workspace WORKSPACE_ID
```

Or install the published public bundle ref:

```bash
mbr extensions install ghcr.io/movebigrocks/mbr-ext-ats:v0.8.23 \
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
