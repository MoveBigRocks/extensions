# ATS V1 Requirements And Roadmap

This document defines the product requirements and delivery plan needed to make
the Move Big Rocks ATS extension coherent, feature-complete, and competitive
with products like Homerun, Teamtailor, Recruitee, Workable, Lever, Ashby, and
Greenhouse.

It is intended to be the working blueprint for implementing the ATS extension as
a real first-party product, not a demo bundle.

Status on April 3, 2026:

- the ATS source now implements the core careers-site baseline from this
  roadmap: generated public careers pages, ATS-owned site/team/gallery/media
  content, ATS-native intake, public resume uploads, general applications,
  talent-pool routing, setup-state tracking, and a recruiter inbox surface
- this document still matters as the parity and differentiation roadmap, but
  the remaining work is now concentrated in full setup-wizard UX, saved
  views/stage-presets productization, structured interviewing, scheduling,
  analytics, governance, and multi-brand or multilingual expansion

## 1. Product Goal

The ATS extension must let a workspace install a recruiting product that:

- feels native inside Move Big Rocks
- generates a polished careers site out of the box
- supports branded customization without requiring code
- reuses MBR primitives consistently
- provides a complete recruiter workflow from first application to decision
- gives operators a guided setup path instead of relying on hidden defaults

The target is not just "enough to publish jobs." The target is a strong V1 that
is credibly as good as, or better than, tools like Homerun for small and
mid-sized teams, while using MBR's shared primitives as an advantage.

## 2. Product Principles

### 2.1 ATS owns recruiting truth

ATS-owned SQL tables are the source of truth for:

- jobs
- candidates
- applications
- stages
- recruiter notes
- careers-site content
- ATS saved views and pipeline presets

### 2.2 MBR primitives are the execution substrate

Move Big Rocks shared primitives are reused intentionally:

- `contacts` for candidate identity
- `cases` for operational recruiting work
- `queues` for routing and work ownership
- `attachments` for resumes and supporting documents
- `tags` for lightweight cross-system labeling
- `automation` for recruiter workflows and follow-up

### 2.3 One canonical user language

The user-facing language must be consistent across setup, admin UI, public site,
automation labels, and projected case data.

Canonical terms:

- `Job`
- `Candidate`
- `Application`
- `Stage`
- `Talent Pool`
- `General Applications`
- `Careers Site`

Internal storage may keep `vacancy` temporarily, but that term must not leak
into the product surface.

### 2.4 Great by default, configurable when needed

The default careers theme must look strong without customization.

The standard customization path is:

- structured content
- uploaded media
- theme tokens

Raw CSS and deeper overrides are advanced features, not the normal workflow.

### 2.5 One canonical intake path

Every application path must resolve to one ATS-native application command.

That includes:

- public careers site apply flow
- optional MBR form adapter
- imports
- future API integrations

No secondary path may create recruiting state differently.

## 3. Competitive Product Bar

The official product/help materials for current ATS competitors show that the
market bar includes:

- polished branded careers sites
- simple onboarding/setup
- public job filters and rich job pages
- talent pools and job alerts
- structured hiring workflows
- recruiter notes and stage management
- interview scheduling
- reporting and funnel analytics
- multi-brand or multi-page employer branding
- candidate communication workflows and automation

To compete well, MBR ATS V1 must meet or exceed the following minimum bar:

- better setup simplicity than enterprise ATS products
- public careers experience comparable to Homerun/Teamtailor/Recruitee
- operator workflow solid enough for daily recruiting
- cleaner cross-product reuse through MBR cases/queues/contacts
- better automation and agent augmentation story than lightweight ATS products

## 4. Scope Definition

### 4.1 In scope for ATS V1

- guided setup wizard
- branded careers site generation
- jobs lifecycle management
- public candidate application flow
- resume upload and attachment handling
- candidate inbox and per-job applications view
- recruiter notes and stage changes
- talent pool and general applications workflow
- ATS saved views and stage presets
- consistent ATS to MBR case projection
- basic structured hiring foundations
- reporting and funnel analytics
- legal/privacy and retention basics
- managed media assets and optional advanced theme overrides

### 4.2 Out of scope for ATS V1

- fully general page-builder / CMS
- full HRIS / payroll / onboarding suite
- enterprise marketplace breadth on day one
- advanced offer management and compensation planning

These can follow later.

## 5. Requirements

## 5.1 Installation And Setup

### Functional requirements

- Installing ATS provisions or validates a dedicated `Hiring` workspace.
- ATS tracks setup progress in persistent ATS-owned state.
- Operators are guided through setup before ATS is considered production-ready.
- Setup is resumable and safe to revisit.
- Setup completion produces a publish-ready careers site and operational ATS
  defaults.

### Setup flow requirements

The setup wizard must include:

1. Workspace confirmation
2. Brand setup
3. Company profile
4. Careers site content
5. Media upload
6. Hiring workflow defaults
7. Candidate experience defaults
8. Legal/privacy configuration
9. Preview and publish

### Data captured during setup

- company name
- careers site title
- tagline
- brand colors
- logo
- hero image
- OG image
- contact email
- company website
- social links
- address
- story copy
- team heading and intro
- gallery heading and intro
- privacy policy URL
- default candidate application settings
- default stage preset
- default queue routing choices
- optional custom CSS enablement

### Acceptance criteria

- A new operator can install ATS and publish a credible branded careers site
  without touching code or manifests.
- ATS exposes setup completeness in the admin experience.
- Incomplete setup produces clear warnings and disables final publish if
  required inputs are missing.

## 5.2 Careers Site

### Functional requirements

- ATS generates a public careers homepage at `/careers`.
- ATS generates a public job page at `/careers/jobs/:slug`.
- Job pages render from ATS SQL data only.
- Changes to ATS careers content or jobs regenerate the public site.
- Public site is responsive and accessible.
- Public site emits SEO metadata and structured data.

### Site-level content requirements

ATS must store and render:

- company name
- site title
- tagline
- meta description
- hero eyebrow
- hero title
- hero body
- primary CTA label and href
- secondary CTA label and href
- story heading
- story body
- jobs heading and intro
- team heading and intro
- gallery heading and intro
- contact email
- website URL
- social URLs
- logo
- hero image
- OG image
- color tokens
- postal address
- privacy policy URL

### Job-level content requirements

Each job must support:

- slug
- title
- team / department
- location
- work mode
- employment type
- status
- summary
- description
- language
- about the role
- responsibilities heading
- responsibilities list
- about you heading
- about you copy
- profile list
- offers heading
- offers intro
- offers list
- quote
- publish date
- optional hero or cover media in later phases

### UX requirements

- Homepage shows job list with filters and empty states.
- Job pages show clear metadata, rich sections, and inline application form.
- Closed or paused jobs show a clear not-accepting state.
- Visitors can navigate back to open roles.
- Talent-community capture and job-alert signup must be supported.

### Branding requirements

- Default theme must look high-quality without customization.
- Operators can customize tokens such as primary, accent, surface, background,
  text, and muted colors.
- Operators can upload or manage logo, hero, team, and gallery images.
- Operators can optionally provide custom CSS overrides.

### Structured data requirements

Job pages must emit `JobPosting` JSON-LD containing at least:

- title
- description
- datePosted
- employmentType
- hiringOrganization
- jobLocation
- jobLocationType where relevant
- public job URL

Homepage should emit organization JSON-LD.

### Acceptance criteria

- A fresh ATS installation can produce a visually credible careers site with
  only structured setup inputs.
- A branded installation can reach inspiration-site quality without code edits.
- Operators do not need to edit bundled assets for normal usage.

## 5.3 Candidate Intake

### Functional requirements

- Public applications are submitted through one ATS-native application command.
- Every successful submission creates:
  - one ATS applicant
  - one ATS application
  - one shared contact
  - one projected recruiting case
- Resume uploads are linked to the application and the case.
- Duplicate candidates are handled safely.

### Data requirements

Applications must track:

- job ID
- applicant ID
- linked contact ID
- linked case ID
- source kind
- source reference ID
- stage
- timestamps
- rejection reason
- linked attachments

Candidate submissions must support:

- full name
- email
- phone
- location
- LinkedIn URL
- portfolio URL
- cover note
- resume attachment ID
- additional screening-question answers in later phases

### Integration requirements

- ATS intake must not depend on a form-specific runtime path.
- Shared MBR forms can still be supported, but only as an adapter into the
  ATS-native pipeline.
- Form-specific fields such as `form_submission_id` are compatibility metadata,
  not the canonical application model.

### Acceptance criteria

- No application path creates different business outcomes.
- A submitted application is always visible consistently in ATS and in the
  projected MBR case.

## 5.4 ATS To MBR Primitive Contract

### Contact projection

- Each candidate must have a linked shared contact record.
- Candidate identity dedupe should primarily key on workspace + email, with
  manual merge support in later phases.

### Case projection

Each application must project to one shared MBR case with:

- recruiting subject
- linked contact
- ATS tags
- ATS custom fields
- linked queue
- linked attachments

### Required ATS tags on created cases

Initial standard set:

- `ats`
- `candidate`
- `applied`
- `job:<slug>`

Additional stage or workflow tags must be system-defined and documented.

### Required ATS custom fields on projected cases

At minimum:

- `ats_job_id`
- `ats_job_slug`
- `ats_job_title`
- `ats_job_status`
- `ats_candidate_id`
- `ats_candidate_email`
- `ats_application_id`
- `ats_application_stage`
- `ats_application_source_kind`
- `ats_application_source_ref_id`
- `ats_resume_attachment_id`
- `ats_careers_path`

### Queue linkage requirements

- Jobs must store a durable `case_queue_id`.
- Queue slug may remain as a readable mirror.
- Queue renames must not break ATS routing.

### Automation requirements

- ATS-native events must be available for application created, stage changed,
  interview scheduled, rejection, hire, and similar lifecycle points.
- ATS automation templates must be keyed on ATS-native conditions rather than
  only `form_slug`.

### Acceptance criteria

- ATS can be understood as an extension with its own domain model while still
  using MBR primitives consistently.
- The case projection contract is stable, documented, and test-covered.

## 5.5 Jobs Lifecycle

### Functional requirements

- Create job
- Edit job
- Publish job
- Pause job
- Close job
- Reopen job
- Archive job

### Publishing requirements

- Publishing a job makes it eligible for public careers-site display.
- Paused jobs remain visible only if explicitly configured, but must not accept
  applications.
- Closed jobs must not accept applications.
- Archived jobs are hidden from public display and no longer mutable except
  through explicit recovery flows.

### Operational requirements

- Job creation provisions a dedicated recruiting queue if needed.
- Jobs can optionally route to general applications or talent pool workflows.

### Acceptance criteria

- Job state is reflected consistently in ATS records, public pages, and case
  routing behavior.

## 5.6 Candidate Operations

### Functional requirements

ATS admin must provide:

- all applications inbox
- per-job applications view
- talent pool view
- general applications view
- candidate detail view
- recruiter note timeline
- stage history
- linked MBR case access
- linked attachments access
- bulk actions

### Candidate detail requirements

Operators must be able to:

- review application details
- open the linked MBR case
- read and add recruiter notes
- move stages
- reject with reason
- mark hired
- withdraw
- add/remove tags
- move candidate to talent pool
- see source and submission metadata

### Saved views requirements

- ATS saved filters must be productized as saved views.
- Views must support stage, job, source, queue, tags, and recency filters.
- Default views should include:
  - Active Funnel
  - Decision Ready
  - Closed Outcomes
  - Needs Review
  - Recently Applied

### Acceptance criteria

- Recruiters can perform daily candidate triage entirely inside ATS without
  falling back to raw cases or direct API calls.

## 5.7 Structured Hiring Foundations

### Functional requirements

ATS V1 must include foundational structured hiring features:

- stage presets
- interview plan definitions
- interviewer assignments
- scorecard or rubric structure
- decision notes

### V1 minimum

The first acceptable slice is:

- interview stages
- interview event records
- interviewer list per stage
- feedback capture
- final decision notes

### Acceptance criteria

- Recruiters can run a consistent evaluation process instead of only moving
  candidates through generic stages.

## 5.8 Scheduling

### Functional requirements

ATS must support at least one scheduling flow in V1. The recommended minimum is:

- create interview scheduling requests
- store interview slots and status
- send candidate self-scheduling links later in the phase
- calendar integration can begin as a staged addition

### Acceptance criteria

- ATS no longer depends on offline/manual scheduling with no system record.

## 5.9 Communications And Talent Community

### Functional requirements

- Public site supports job alerts or talent-community signup.
- ATS supports general applications.
- ATS supports moving a candidate into talent pool.
- ATS supports basic candidate communication templates.
- ATS supports nurture or follow-up automation in later iterations.

### Acceptance criteria

- ATS is not only a requisition tracker; it can hold candidate relationships
  beyond one open job.

## 5.10 Reporting And Analytics

### Functional requirements

ATS must provide:

- open jobs count
- application volume by job
- application volume by source
- stage funnel
- stage aging
- time to review
- time to hire
- recruiter workload snapshots
- careers-site conversion basics

### Acceptance criteria

- Operators can answer basic recruiting performance questions from ATS without
  raw SQL or external exports.

## 5.11 Governance And Compliance

### Functional requirements

- audit trail for job, candidate, and stage changes
- role-based access control for sensitive actions
- privacy policy configuration
- retention and delete/anonymize candidate records later in the phase
- clear file handling rules for candidate attachments

### Acceptance criteria

- ATS is operationally safe for real hiring workflows.

## 6. Data Model Requirements

## 6.1 Existing owned tables to keep

- `vacancies`
- `applicants`
- `applications`
- `recruiter_notes`
- `stage_presets`
- `saved_filters`
- `careers_site_profiles`
- `careers_team_members`
- `careers_gallery_items`

## 6.2 Required schema changes

### Jobs

Add or evolve:

- `case_queue_id`
- optional richer department/location/job-type references later
- optional media references for job-specific visual content later

### Applications

Add:

- `source_kind`
- `source_ref_id`
- optional `current_queue_id` mirror if needed
- audit metadata

Keep `form_submission_id` only as compatibility metadata.

### Careers site profile

Add:

- `privacy_policy_url`
- optional `custom_css`
- optional `careers_domain`
- optional `is_published`
- setup status or onboarding completion references

### Setup / onboarding

Add ATS-owned setup state table(s), for example:

- `setup_state`
- `setup_checklist_items`

### Structured hiring

Add later-phase tables for:

- interview plans
- interviews
- scorecards / feedback
- decision records

### Analytics

Prefer query-driven reporting first.

Add materialized or snapshot tables only if scale/performance requires them.

## 7. API Requirements

## 7.1 Setup APIs

Required endpoints:

- get setup state
- update setup section
- upload branding/media assets
- validate setup completeness
- finalize onboarding

## 7.2 Careers APIs

Required endpoints:

- get careers bundle
- update site profile
- update team
- update gallery
- update legal / policy settings
- publish / unpublish careers site
- preview generated site state
- upload/remove managed assets
- update theme/custom CSS overrides

## 7.3 Jobs APIs

Required endpoints:

- list jobs
- create job
- update job
- publish job
- pause job
- close job
- reopen job
- archive job
- list applications by job

## 7.4 Candidate APIs

Required endpoints:

- submit application
- upload application attachment
- list candidates/applications
- get candidate detail
- add recruiter note
- change stage
- bulk stage update
- move to talent pool
- reject
- hire
- withdraw
- merge duplicates later

## 7.5 Analytics APIs

Required endpoints:

- summary dashboard
- funnel data
- source performance
- per-job performance
- stage aging

## 8. UI Requirements

## 8.1 ATS admin information architecture

The operator experience should be organized into:

- Setup
- Careers Site
- Jobs
- Candidates
- Talent Pool
- Views
- Analytics
- Settings

## 8.2 Setup UI

Must provide:

- guided wizard
- progress meter
- checklist
- inline validation
- preview before publish

## 8.3 Careers Site UI

Must provide:

- brand controls
- media upload
- company/profile copy editor
- team CRUD
- gallery CRUD
- theme token editing
- optional advanced overrides
- preview and publish controls

## 8.4 Jobs UI

Must provide:

- list with status
- filters
- create/edit forms
- publish/pause/close/archive actions
- applications count per job

## 8.5 Candidates UI

Must provide:

- all applications view
- per-job view
- stage controls
- notes
- attachments
- linked case access
- bulk actions
- saved views

## 8.6 Analytics UI

Must provide:

- KPI summary cards
- stage funnel
- source mix
- per-job breakdown
- aging indicators

## 9. Non-Functional Requirements

- public careers site must be responsive and accessible
- setup and admin flows must be understandable by non-technical operators
- application intake must be idempotent enough to avoid accidental duplication
- file upload limits and content-type validation must be explicit
- public endpoints must be rate-limited and abuse-resistant
- generated site publishing must be reliable and observable
- schema migrations must be safe for existing installs

## 10. Delivery Plan

## 10.1 Phase A: Coherence Foundation

### Goals

- lock the domain contract
- remove overlapping intake models
- standardize ATS to MBR projection

### Work items

- define canonical user language
- add `case_queue_id`
- add `source_kind` and `source_ref_id`
- downgrade `form_submission_id` to compatibility-only
- standardize ATS custom fields mirrored to cases
- standardize ATS tags
- update automation rules to ATS-native triggers
- document projection contract
- migrate existing records

### Exit criteria

- ATS has one canonical intake model
- projected cases are consistent and test-covered
- queue renames cannot break job routing

## 10.2 Phase B: Setup And Careers Site GA

### Goals

- make ATS installable by normal operators
- deliver public-site quality at the level of the inspiration reference

### Work items

- add setup state tables
- add onboarding endpoints
- build setup wizard UI
- add media upload support for brand assets
- add privacy policy and legal content support
- add custom CSS override support
- improve homepage/job-page filtering and empty states
- add talent-community/job-alert capture
- complete publish/unpublish behavior

### Exit criteria

- a new workspace can install ATS, complete setup, and publish a credible site
  without touching code

## 10.3 Phase C: Candidate Operations GA

### Goals

- make ATS usable as a day-to-day recruiting tool

### Work items

- build candidates inbox
- build candidate detail view
- build notes and stage UI
- expose talent pool and general applications
- build saved views UI
- add bulk actions
- add duplicate-candidate handling

### Exit criteria

- recruiters can work entirely in ATS for normal candidate review

## 10.4 Phase D: Structured Hiring And Reporting

### Goals

- move beyond basic pipeline tracking

### Work items

- interview plans
- interview records
- feedback capture
- decision support
- reporting endpoints and dashboards
- stage aging and funnel analytics

### Exit criteria

- ATS supports a repeatable, measurable recruiting process

## 10.5 Phase E: Differentiation

### Goals

- exceed lightweight ATS competitors
- use MBR as a product advantage

### Work items

- multi-brand support
- multilingual careers pages
- internal job board
- referral support
- richer automations and nurture flows
- AI-assisted setup, job drafting, candidate summaries, and recruiter workflows

### Exit criteria

- ATS is meaningfully stronger than a careers-site ATS alone

## 11. Implementation Workstreams

## 11.1 Schema and migrations

- add setup state tables
- add queue ID references
- add application source fields
- add site profile fields for legal/customization
- add structured hiring tables later
- backfill existing installs safely

## 11.2 Runtime and domain

- update domain types
- canonicalize application creation flow
- update projection logic to cases
- extend analytics queries
- extend publish/render logic

## 11.3 Platform integration

- ensure resolved extension context continues to flow to all public/admin
  endpoints
- add managed asset workflows where needed
- ensure extension install/upgrade seeds setup defaults correctly

## 11.4 Admin UI

- setup wizard
- careers editor improvements
- jobs workspace
- candidates workspace
- analytics workspace

## 11.5 Public UX

- upgrade careers homepage and job pages
- talent-community capture
- improved application feedback states
- legal/footer completeness

## 11.6 Testing

- schema migration tests
- service tests
- HTTP integration tests
- projection contract tests
- publish/render tests
- end-to-end install/setup/publish/apply/review tests

## 12. Acceptance Checklist

ATS V1 is considered complete when all of the following are true:

- ATS can be installed by an operator and completed through setup
- careers branding and content are managed through structured ATS data
- public careers pages look polished without manual template edits
- media assets can be managed through ATS
- applications always create ATS records plus consistent MBR projections
- recruiter workflows are available in the ATS UI
- general applications and talent pool are real product surfaces
- saved views and stage presets are usable in the UI
- analytics answer the basic operational questions
- automation is ATS-native rather than form-native
- the data model is coherent and documented
- the user-facing language is consistent across setup, admin, and public surfaces

## 13. Immediate Execution Order

The recommended implementation order is:

1. Phase A: Coherence Foundation
2. Phase B: Setup And Careers Site GA
3. Phase C: Candidate Operations GA
4. Phase D: Structured Hiring And Reporting
5. Phase E: Differentiation

This order ensures the team does not build more UX on top of a shaky model.

## 14. Benchmark References

The planning for this document was informed by the publicly available product
and help materials for:

- Homerun
- Teamtailor
- Recruitee
- Workable
- Lever
- Ashby
- Greenhouse

The intent is not to clone any one competitor. The intent is to meet the best
parts of their bar while using MBR's shared primitives and agent/automation
capabilities to build a more coherent operating system for hiring.
