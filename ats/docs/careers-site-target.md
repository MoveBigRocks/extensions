# ATS Careers Site Target

This document captures what the inspiration site in the sibling `careers/`
folder actually contains, how that compares to the current ATS extension, and
what the ATS extension needs in order to generate a comparable careers site
from structured ATS data instead of hard-coded demo HTML.

Status on April 4, 2026:

- the ATS source in this repo renders a real `/careers` homepage and
  `/careers/jobs/:slug` detail pages from ATS-owned SQL data
- the extension owns site profile, team, gallery, managed media assets,
  privacy/custom CSS settings, general-application intake, and talent-pool
  routing
- only open jobs render as active public job pages; draft jobs are omitted and
  previously published closed or paused jobs are replaced with unavailable pages
- the remaining gaps are mostly parity-plus product areas such as richer public
  filtering/catalog controls, multilingual support, custom domains, and the
  broader recruiter-suite features tracked in the roadmap docs

## Reference Site Inventory

The inspiration implementation has two distinct content layers.

### Site-level content

- Sticky header with configurable logo and social links
- Hero section with:
  - hero headline
  - multi-paragraph company story
  - primary CTA
  - branded background image / overlay
- Homepage cover image
- Job openings list
- Team grid
- Workplace gallery
- Footer with:
  - company address
  - contact email
  - company website link
  - social links
  - privacy policy link

### Job-level content

Each vacancy has both listing metadata and long-form public content.

- `title`
- `date`
- `type`
- `department`
- `location`
- `summary`
- `language`
- `about_the_job`
- `responsibilities_heading`
- `responsibilities[]`
- `about_you_heading`
- `about_you`
- `profile[]`
- `offers_heading`
- `offers_intro`
- `offers[]`
- `quote`

The site template also assumes a small set of shared job-page assets:

- job header image
- job cover image
- job gallery images

### Structured data in the inspiration site

The reference job pages emit `schema.org/JobPosting` JSON-LD with:

- `title`
- `description`
- `datePosted`
- `employmentType`
- `hiringOrganization.name`
- `hiringOrganization.sameAs`
- `hiringOrganization.logo`
- `jobLocation.address.streetAddress`
- `jobLocation.address.addressLocality`
- `jobLocation.address.addressRegion`
- `jobLocation.address.postalCode`
- `jobLocation.address.addressCountry`

The archived Homerun snapshot also exposes a structured vacancy catalog on the
homepage:

- vacancies:
  - `id`
  - `title`
  - `location_id`
  - `department_id`
  - `job_type_id`
  - `url`
  - presentation ordering fields
- departments:
  - `id`
  - `name`
  - `position`
- locations:
  - `id`
  - `name`
  - `address`
  - `locality`
  - `region`
  - `postal_code`
  - `country`
- job types:
  - `id`
  - `name`
  - `position`

## ATS Extension Today

The ATS extension now owns the baseline needed to generate a strong careers
site from structured data:

- ATS-owned vacancy lifecycle
- applicant and application records
- candidate stages and recruiter notes
- generated `/careers` and `/careers/jobs/:slug` public pages
- `schema.org/JobPosting` JSON-LD
- ATS-native public application ingest and resume upload
- ATS-owned site profile, team, gallery, setup state, and managed media assets
- general-application intake and talent-pool routing

The important public-content upgrade that unlocked the richer job detail pages
is the structured vacancy-copy model:

- `language`
- `aboutTheJob`
- `responsibilities[]`
- `responsibilitiesHeading`
- `aboutYou`
- `aboutYouHeading`
- `profile[]`
- `offersIntro`
- `offers[]`
- `offersHeading`
- `quote`

These fields now live in the ATS vacancy model and database, which lets the
extension render job pages in the same shape as the inspiration site instead of
squeezing everything into one long description.

The same is now true for operator-configurable site content that does not
belong on a single vacancy:

- company name and description
- main website
- privacy policy URL
- contact email
- social links
- postal address
- tagline
- default location metadata
- hero headline / body / CTA copy

These fields now live in ATS-owned careers-site tables instead of being forced
through flat extension config.

The reference site also depends on reusable collections that ATS now owns:

- team members:
  - name
  - role
  - image URL / asset path
  - social URL
- gallery items:
  - image URL / asset path
  - slot or placement

The public publishing/runtime layer is also now in place:

- homepage built from published vacancies plus site profile content
- job detail page route per vacancy slug
- JSON-LD emission from ATS data
- inline application form on the job page
- branded assets and image references

## Remaining Gaps Against Full Parity

The original site-shape gap is largely closed in the ATS source. The remaining
careers-site gaps are now higher-order product concerns:

- richer public filtering and catalog dimensions beyond the current ATS job
  metadata
- multilingual content and localized careers pages
- custom-domain and broader publishing controls
- deeper analytics, job alerts, and candidate-marketing surfaces

## Practical Mapping From Inspiration Site To ATS

### Vacancy fields

| Inspiration field | ATS source |
| --- | --- |
| `title` | `vacancies.title` |
| `department` | `vacancies.team` |
| `location` | `vacancies.location` |
| `type` | `vacancies.employment_type` |
| `summary` | `vacancies.summary` |
| `about_the_job` | `vacancies.public_about_the_job` |
| `responsibilities[]` | `vacancies.public_responsibilities` |
| `about_you` | `vacancies.public_about_you` |
| `profile[]` | `vacancies.public_profile` |
| `offers_intro` | `vacancies.public_offers_intro` |
| `offers[]` | `vacancies.public_offers` |
| `quote` | `vacancies.public_quote` |

### Site profile fields

| Inspiration field | Recommended ATS source |
| --- | --- |
| company name | `careers_site_profiles.company_name` |
| company website | `careers_site_profiles.company_website` |
| privacy policy | `careers_site_profiles.privacy_policy_url` |
| contact email | `careers_site_profiles.contact_email` |
| address | `careers_site_profiles.address_*` |
| tagline | `careers_site_profiles.tagline` |
| hero copy | `careers_site_profiles.hero_*` |
| team grid | `careers_team_members` |
| gallery | `careers_gallery_items` |

## Current Recommendation

The ATS extension should move toward a generated public careers product with:

1. ATS-owned structured vacancy content
2. ATS-owned site-profile publishing data
3. runtime-rendered or generated public pages
4. asset-backed branding and imagery

That will let the extension produce a site in the shape of the inspiration
implementation while keeping the branding configurable per installation.
