# ATS Gap-Closure Implementation Plan

This document translates the ATS V1 product requirements into a detailed
implementation plan that closes the gap between the current ATS extension and
the target state.

It is intentionally execution-oriented. It focuses on what exists today, what
must change, how the work should be sequenced, and what each milestone must
ship in order to make the ATS extension coherent, feature-complete, and
competitive.

See also:

- [`ats-v1-requirements-and-roadmap.md`](./ats-v1-requirements-and-roadmap.md)
- [`careers-site-target.md`](./careers-site-target.md)

Status on April 3, 2026:

- the source in this repo has now implemented the original coherence baseline:
  ATS-native public intake, durable ATS-owned careers-site state, generated
  `/careers` pages, managed media publishing, general applications, and
  talent-pool routing
- the sections below still describe the execution model and milestone ordering,
  but the remaining gaps are now the deeper operator, setup, analytics, and
  governance layers rather than the first public-site foundation

## 1. Objective

Move ATS from the current implemented baseline:

- strong ATS-owned schema foundation
- generated public careers site and job detail pages
- ATS-native public application intake and resume uploads
- ATS-owned site, team, gallery, managed media, and setup state
- good reuse of MBR cases, contacts, queues, attachments, tags, and automation
- working candidate inbox plus stage, note, and routing actions
- remaining operator/setup/governance depth still to ship

to the target state:

- coherent ATS domain model
- consistent MBR primitive reuse contract
- one canonical user language
- one canonical application intake path
- setup wizard and publish-ready defaults
- high-quality public careers site with structured branding/content/media inputs
- complete recruiter workflow
- talent pool and general applications as real product surfaces
- structured hiring foundations
- analytics and operational governance

## 2. Current State Summary

## 2.1 What exists now

### ATS-owned SQL schema

The extension currently owns:

- `vacancies`
- `applicants`
- `applications`
- `recruiter_notes`
- `stage_presets`
- `saved_filters`
- `careers_site_profiles`
- `careers_team_members`
- `careers_gallery_items`

### ATS runtime and public surface

The runtime currently supports:

- jobs CRUD and status transitions
- generated `/careers`
- generated `/careers/jobs/:slug`
- public application ingest via `/careers/applications`
- public resume upload via `/careers/attachments`
- public general-application intake via a hidden ATS-owned intake role
- recruiter notes and stage changes
- candidate routing between job queues and the shared talent pool
- careers site regeneration on ATS content changes
- managed careers-media upload and publication

### Shared MBR primitives already reused

The current runtime already projects ATS workflows onto:

- contacts
- cases
- queues
- attachments
- tags
- automation

### Admin surface

The current admin surface lets operators edit:

- site profile
- team
- gallery
- jobs
- setup checklist state
- candidate inbox with stage, note, and routing actions

## 2.2 What is missing now

- fully guided setup wizard UX
- dedicated candidate-detail timeline and richer linked-case surface
- saved views UI
- stage-preset UI
- structured hiring entities and workflows
- scheduling flows
- analytics UI and APIs
- governance/audit/retention features
- multi-brand, multilingual, and custom-domain publishing depth

## 2.3 Architectural drift to resolve first

These are the highest-risk inconsistencies in the current model:

- the ATS-to-case projection contract now exists, but it still needs to be
  tightened into a stable operator-facing contract and surfaced consistently in
  the UI
- saved views, stage presets, and setup state exist in ATS-owned storage, but
  are not yet fully productized as polished operator surfaces
- broader recruiter-suite features like scheduling, analytics, and governance
  remain outside the current baseline

## 3. Design Decisions To Lock Before Implementation

These decisions should be treated as implementation constraints.

### 3.1 ATS-native intake is canonical

All applications must resolve through a single ATS-native application command.

Allowed adapters:

- public careers form
- MBR shared form
- API/import adapters

No adapter may create recruiting state differently from the canonical flow.

### 3.2 ATS schema owns recruiting truth

ATS SQL is the source of truth for:

- jobs
- candidates
- applications
- recruiter notes
- stage state
- ATS views/presets
- careers-site content
- setup state

### 3.3 MBR primitives are projections and workflow infrastructure

MBR shared primitives are not the recruiting source of truth.

They are the execution layer:

- shared contact per candidate
- shared case per application
- shared queue for routing
- shared attachments for resumes and media backing where appropriate
- shared tags
- shared automation

### 3.4 User-facing language is fixed

Use:

- `Job`
- `Candidate`
- `Application`
- `Stage`
- `Talent Pool`
- `General Applications`

Do not expose `vacancy` in the UI or public site.

### 3.5 Careers site customization model

The default customization model is:

- structured content fields
- managed media uploads
- theme tokens

Advanced customization adds:

- optional custom CSS

No general page builder is required for V1.

### 3.6 Branding asset model

Recommended implementation:

- binary media stored via shared attachment infrastructure
- ATS-owned media metadata table tracks semantic usage
- public publish step copies or republishes chosen assets into the extension
  website artifact surface

This keeps media compatible with MBR primitives while still producing stable
static public assets for the generated careers site.

## 4. Gap Matrix

| Gap | Current state | Target state | Closure milestone |
| --- | --- | --- | --- |
| Setup flow | none | guided onboarding wizard | M2 |
| Intake contract | ATS-native runtime plus lingering form-native contract | one canonical ATS-native flow | M1 |
| Queue linkage | slug only | durable queue ID + slug mirror | M1 |
| Branding source of truth | ATS SQL plus overlapping extension config | ATS SQL canonical, config seeds only | M1/M2 |
| Managed media | URL-only inputs | uploaded managed assets | M2 |
| Public careers quality | good but basic | inspiration-grade, configurable, complete | M2 |
| Candidate ops UI | backend support only | real inbox/detail workflow | M3 |
| Talent pool/general applications | seeded queues only | real ATS workflow surfaces | M3 |
| Saved views/stage presets | backend support only | usable admin UI | M3 |
| Structured hiring | minimal stage model | interview plans and feedback foundations | M4 |
| Scheduling | none | first-class scheduling workflow | M4 |
| Analytics | minimal or absent | recruiter and hiring reporting | M5 |
| Governance | limited | audit, permissions, retention basics | M5 |
| Differentiation | none yet | multi-brand, multilingual, AI/agents | M6 |

## 5. Delivery Strategy

Deliver the work in ordered milestones.

Each milestone should leave the product in a shippable state. Later milestones
should not depend on unresolved domain ambiguity from earlier ones.

Recommended milestone order:

- M0: specification freeze and inventory
- M1: coherence foundation
- M2: setup and careers site GA
- M3: recruiter operations GA
- M4: structured hiring and scheduling foundations
- M5: analytics, governance, and hardening
- M6: differentiation

## 6. Milestone M0: Specification Freeze And Inventory

## Goals

- lock core terminology
- lock ownership boundaries
- document projection contract
- inventory current code paths, migrations, and UI dependencies

## Deliverables

- final domain language glossary
- final ATS-to-MBR projection spec
- final intake contract spec
- final setup flow outline
- implementation backlog seeded from this document

## Work items

### Documentation

- finalize the user-facing glossary
- document every ATS custom field mirrored into cases
- document ATS tag conventions
- document queue ownership and routing semantics
- document what seeded forms still mean after intake canonicalization

### Inventory

- list every current endpoint and classify as keep/change/deprecate
- list every current schema field and classify as canonical/compatibility/dead
- list every current admin page and determine whether it is primary or legacy
- list every seeded queue/form/rule and determine intended post-M1 role

## Exit criteria

- the team has a stable contract for M1 implementation

## 7. Milestone M1: Coherence Foundation

## Goals

- eliminate architectural drift
- standardize application creation and case projection
- make shared primitive reuse explicit and stable

## 7.1 Schema changes

### Jobs

Add:

- `case_queue_id TEXT NOT NULL DEFAULT ''`

Keep:

- `case_queue_slug` as a mirror

Consider later deprecation:

- `application_form_slug`

### Applications

Add:

- `source_kind TEXT NOT NULL DEFAULT 'ats_public'`
- `source_ref_id TEXT NOT NULL DEFAULT ''`
- optional audit metadata if not already covered elsewhere

Keep:

- `form_submission_id` as compatibility metadata only

### Careers site profile

Add:

- `privacy_policy_url TEXT NOT NULL DEFAULT ''`
- `custom_css TEXT NOT NULL DEFAULT ''`
- `custom_css_enabled BOOLEAN NOT NULL DEFAULT FALSE`
- `published_at TIMESTAMPTZ`

### Setup state

Add:

- `setup_state` table keyed by workspace
- `setup_steps` or serialized checklist state

Suggested fields:

- `workspace_id`
- `status`
- `current_step`
- `completed_steps JSONB`
- `last_completed_at`
- `created_at`
- `updated_at`

### Media metadata

Add:

- `careers_media_assets`

Suggested fields:

- `id`
- `workspace_id`
- `usage_kind`
- `attachment_id`
- `artifact_path`
- `alt_text`
- `caption`
- `display_order`
- `is_active`
- `created_at`
- `updated_at`

## 7.2 Runtime/domain changes

### Canonical intake flow

- introduce one internal `SubmitApplication` orchestration path that all public,
  form, import, and future integration flows call
- adapt existing `/careers/applications` to call that path
- build a form adapter path rather than a separate business flow

### Case projection contract

Standardize case projection to always set:

- queue ID
- tags
- ATS custom fields
- linked contact
- linked attachment references
- canonical recruiting category

### Queue resolution

- create jobs with queue object resolution returning both queue ID and queue slug
- backfill existing jobs from slug to queue ID
- submit applications using queue ID, not queue slug

### Source tracking

- populate `source_kind` and `source_ref_id` for every application
- keep `form_submission_id` populated only when a form adapter exists

### Automation events

Add or standardize ATS-native events:

- `ats_application_created`
- `ats_application_stage_changed`
- `ats_job_published`
- `ats_job_closed`
- `ats_candidate_moved_to_talent_pool`

### Config ownership

- stop reading live careers branding from installed extension config
- use config only to seed ATS SQL defaults at install/upgrade time

## 7.3 Manifest and seeding changes

- keep seeded form only if it is repurposed as an adapter to canonical ATS intake
- update seeded automation rules to ATS-native conditions
- preserve seeded queues but document their operational purpose
- update manifest copy so it no longer implies form-native public intake

## 7.4 Admin/API changes

- add endpoint to expose the case projection contract and setup state if needed
- add versioned or documented deprecation notice for old fields/endpoints where
  needed

## 7.5 Migration/backfill tasks

- add migration for new job/application/profile/setup/media fields
- backfill `case_queue_id` by resolving existing queue slugs
- backfill `source_kind` for existing applications as `legacy_runtime` or
  `form_submission` where determinable
- mark compatibility fields in code comments and docs

## 7.6 Test plan

- migration tests for queue ID and source backfills
- service tests for canonical intake
- service tests for case projection invariants
- HTTP tests for public submit and legacy form adapter path
- automation tests for new ATS-native triggers

## Exit criteria

- ATS has one intake model
- ATS case projection is stable and documented
- ATS branding/content source of truth is unambiguous
- no core path depends on queue slugs or form semantics alone

## 8. Milestone M2: Setup And Careers Site GA

## Goals

- make ATS easy to adopt
- make the public site high quality and easy to configure

## 8.1 Setup implementation

### Backend

- add setup state service methods
- add setup endpoints:
  - `GET /extensions/ats/api/setup`
  - `PUT /extensions/ats/api/setup/:step`
  - `POST /extensions/ats/api/setup/complete`
- validate required setup fields by step
- expose checklist completeness and publish readiness

### Frontend

- build setup wizard page or add setup mode to the ATS admin
- implement step progression
- persist step completion
- allow revisiting completed steps
- show preview links and publish readiness

### Setup steps to implement first

1. Workspace and product naming
2. Brand and colors
3. Company profile and contact details
4. Careers-site content
5. Media upload
6. Recruiting defaults
7. Legal/privacy
8. Preview/publish

## 8.2 Managed media implementation

### Backend

- add careers media upload endpoint
- store uploads through attachment infrastructure
- create ATS media metadata records
- on publish, republish media into website artifact surface under stable paths

Suggested endpoints:

- `POST /extensions/ats/api/careers/assets`
- `DELETE /extensions/ats/api/careers/assets/:id`
- `GET /extensions/ats/api/careers/assets`

### Frontend

- replace URL-only inputs with upload-capable media pickers
- allow choosing uploaded or external media
- show asset preview
- allow reordering/removal where relevant

## 8.3 Careers-site model completion

Add support for:

- privacy policy URL
- footer legal links
- custom CSS override
- richer jobs filtering on homepage
- talent-community/job-alert signup capture
- per-job optional media in a later sub-slice if needed

## 8.4 Careers-site rendering improvements

- complete footer information
- add public job filtering UI
- add job-alert/talent community capture area
- ensure success/error states for public applications are polished
- ensure accessibility and responsive behavior
- include optional custom CSS if enabled

## 8.5 Publish model

- make publish/unpublish explicit in ATS setup/settings
- track last published timestamp
- preserve auto-regeneration on content changes while still allowing operators to
  understand what is live

## 8.6 Test plan

- setup-state tests
- upload/publish tests for media assets
- render tests covering legal/footer/custom CSS
- E2E tests for install -> setup -> publish -> public preview

## Exit criteria

- a non-technical operator can install ATS, complete setup, upload branding
  assets, preview, and publish a strong careers site

## 9. Milestone M3: Recruiter Operations GA

## Goals

- make ATS usable for everyday recruiting work

## 9.1 Candidate inbox and detail model

### Backend

- add list APIs for:
  - all applications
  - applications by job
  - talent pool
  - general applications
  - candidate detail
- add filtering and pagination support

Suggested endpoints:

- `GET /extensions/ats/api/applications`
- `GET /extensions/ats/api/applications/:id`
- `GET /extensions/ats/api/talent-pool`
- `GET /extensions/ats/api/general-applications`

### Frontend

- build applications inbox
- build candidate detail panel/page
- show recruiter notes
- show stage timeline
- show linked case and attachments
- support note creation and stage changes in UI

## 9.2 Saved views and stage presets UI

### Backend

- add CRUD endpoints for saved views and stage presets if needed

### Frontend

- saved views selector
- default views seeded from ATS tables
- stage preset management UI

## 9.3 Talent pool and general applications

### Backend

- formalize routing rules for:
  - job-specific queue
  - general-applications queue
  - talent-pool queue
- expose actions to move candidates between workflows

### Frontend

- show talent-pool and general-applications views as first-class surfaces
- add move-to-talent-pool action
- add assign-to-job or return-to-pipeline action later

## 9.4 Bulk actions

Add bulk capabilities:

- move stage
- add/remove tags
- move to talent pool
- archive
- reject

## 9.5 Candidate dedupe

### V1 minimum

- detect duplicates by workspace + email
- surface duplicate warning in the UI

### Later in milestone

- merge candidate records across multiple applications

## 9.6 Test plan

- inbox filtering tests
- candidate detail tests
- talent pool/general applications flow tests
- saved view tests
- bulk action tests
- duplicate detection tests

## Exit criteria

- recruiters can work daily in ATS without falling back to raw cases or API
  calls

## 10. Milestone M4: Structured Hiring And Scheduling Foundations

## Goals

- move ATS beyond simple pipeline tracking

## 10.1 Structured hiring schema

Add tables for:

- interview plans
- interview stages
- interview records
- interviewer assignments
- feedback / scorecards
- decision notes

## 10.2 Structured hiring UI

Build:

- interview plan editor per job
- interview status view per candidate
- interviewer assignment controls
- feedback capture UI
- final decision notes UI

## 10.3 Scheduling

### V1 minimum slice

- create interview schedule records
- store date/time/status
- send candidate scheduling links in later slice

### Recommended next slice

- calendar provider integration
- self-scheduling or slot selection
- interview reminders and rescheduling

## 10.4 Automation

Add ATS workflow templates for:

- interview scheduled
- interview feedback overdue
- stage inactivity
- candidate follow-up

## 10.5 Test plan

- structured hiring schema tests
- interview workflow service tests
- scheduling API tests
- automation trigger tests

## Exit criteria

- ATS supports a disciplined interview process with system records and recruiter
  support

## 11. Milestone M5: Analytics, Governance, And Hardening

## Goals

- make ATS operationally safe and measurable

## 11.1 Analytics

### Backend

Add reporting services/endpoints for:

- jobs open count
- applications by stage
- applications by source
- stage aging
- time to review
- time to hire
- per-job funnel
- recruiter workload
- careers-site conversion basics

### Frontend

- dashboard overview
- funnel charts
- per-job detail
- aging indicators

## 11.2 Governance

Add:

- audit trail surfaces
- permission gating for sensitive actions
- retention configuration
- privacy and delete/anonymize candidate operations
- clear upload/type/size enforcement rules

## 11.3 Hardening

- rate-limit public endpoints appropriately
- add idempotency or duplicate-submit protection
- expand migration coverage
- expand end-to-end test coverage
- add observability for publish failures and intake failures

## 11.4 Test plan

- analytics query correctness tests
- retention and delete tests
- permission tests
- observability and failure-path tests

## Exit criteria

- ATS can be operated safely and measured effectively in production

## 12. Milestone M6: Differentiation

## Goals

- use MBR and agent capabilities to exceed lightweight ATS products

## Work items

- multi-brand support
- multilingual careers pages
- internal job board
- referrals
- nurture sequences
- AI-assisted setup
- AI-assisted job drafting
- AI candidate summaries
- recruiter task automation and follow-up generation

## Exit criteria

- ATS is not only competitive; it has clear product advantages over simpler ATS
  tools

## 13. Implementation Workstreams

The milestones above should be implemented through parallel workstreams where
possible.

## 13.1 Workstream A: Schema and migrations

Deliver:

- new migrations
- safe backfills
- compatibility shims

## 13.2 Workstream B: Domain/runtime services

Deliver:

- canonical intake command
- projection contract
- setup state services
- analytics services
- scheduling and structured hiring services later

## 13.3 Workstream C: Public site generation

Deliver:

- render improvements
- media publishing
- legal/footer completeness
- custom CSS and advanced styling support

## 13.4 Workstream D: Admin UI

Deliver:

- setup wizard
- careers editor improvements
- recruiter operations surfaces
- analytics dashboards

## 13.5 Workstream E: Platform integration

Deliver:

- install/upgrade seeding behavior
- extension config seeding rules
- public/admin route context correctness
- managed asset plumbing

## 13.6 Workstream F: Testing and rollout

Deliver:

- migration test suite
- service + HTTP tests
- install/setup/apply/review E2Es
- rollout and upgrade playbooks

## 14. Proposed PR / Slice Breakdown

This is the recommended order for concrete engineering slices.

## Slice 1: Domain coherence migration

- add queue ID and source fields
- add setup/media/profile fields
- backfill existing data
- update models

## Slice 2: Canonical intake and projection contract

- unify application creation flow
- add ATS-native automation triggers
- standardize case tags/custom fields
- de-emphasize form-native path

## Slice 3: Setup backend

- setup state tables and APIs
- setup validation logic
- readiness model

## Slice 4: Media and publish pipeline

- media upload API
- media metadata
- website artifact publishing for uploaded assets
- custom CSS publishing

## Slice 5: Setup UI

- onboarding wizard
- checklist
- preview/publish flow

## Slice 6: Careers editor upgrade

- swap URL-only inputs for media pickers
- add privacy/legal controls
- improve public-site editor UX

## Slice 7: Applications inbox and candidate detail

- inbox APIs
- candidate detail APIs
- recruiter workflow UI

## Slice 8: Talent pool / general applications / saved views

- backend routing and actions
- UI surfaces and actions

## Slice 9: Structured hiring

- interview model
- feedback model
- UI

## Slice 10: Scheduling

- schedule records
- integration hooks
- UI and automation

## Slice 11: Analytics and governance

- reporting APIs and UI
- audit and retention basics

## Slice 12: Differentiation

- multi-brand
- multilingual
- AI/agent enhancements

## 15. Rollout And Compatibility Plan

## 15.1 Backward compatibility

Until M1 is complete:

- keep compatibility support for current fields and flows
- mark compatibility-only fields clearly in code and docs

After M1:

- old paths remain supported where safe
- new internal services become canonical

## 15.2 Upgrade path

For existing ATS installs:

- run migrations
- backfill queue IDs
- backfill source metadata
- seed setup state from existing site profile content
- preserve published careers site behavior

## 15.3 Release gates

Do not call ATS V1 complete until:

- M1, M2, and M3 are done
- setup works
- candidate operations UI exists
- talent pool/general applications are real
- source-of-truth drift is resolved

Do not call ATS competitive with Homerun et al until:

- M4 and M5 are done

Do not call ATS better than lightweight ATS competitors until:

- meaningful M6 differentiation ships

## 16. Test Matrix

Every milestone must extend the test matrix.

## 16.1 Required coverage areas

- schema migrations
- service-layer invariants
- HTTP endpoints
- setup state transitions
- careers site render output
- public submit flow
- attachment upload/linking
- case projection correctness
- automation rule triggers
- candidate workflow actions
- analytics correctness
- upgrade/backfill safety

## 16.2 End-to-end proof flows

At minimum:

1. install ATS
2. complete setup
3. upload brand assets
4. publish careers site
5. create and publish a job
6. apply publicly with resume
7. verify ATS records plus projected contact/case/queue/attachment
8. move candidate through stages
9. add notes
10. move candidate to talent pool
11. verify analytics snapshots

## 17. Definition Of Done

The current gap between ATS and the target state is considered closed when:

- ATS domain ownership is coherent
- MBR primitive reuse is consistent and documented
- setup exists and is polished
- careers site customization works through structured content and managed media
- public careers output is high quality
- candidate operations are available in the admin UI
- talent pool and general applications are real product surfaces
- structured hiring foundations exist
- analytics and governance basics exist
- compatibility drift from the old model has been retired or isolated

At that point, the ATS extension will be able to credibly compete with
Homerun-style ATS products on simplicity and public-site quality while also
benefiting from MBR's stronger workflow, automation, and cross-product
primitives.
