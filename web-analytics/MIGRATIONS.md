# Web Analytics Migration Boundary

Owned schema:

- `ext_demandops_web_analytics`

Canonical migration files:

- `migrations/000001_init.up.sql`
- `migrations/000002_rls.up.sql`
- `migrations/000003_event_visit_model_v2.up.sql`

What belongs in these migrations:

- analytics properties
- hostname rules
- goals
- salts
- raw events
- sessions
- v2 event metadata and visit attribution fields
- indexes and RLS policies for the owned schema

What does not belong in these migrations:

- the tracking script asset
- endpoint registration and admin navigation
- seeded properties or goals
- data import/export jobs
- one-off operational repair scripts

Schema rules:

- workspace-scoped tables carry both `workspace_id` and `extension_install_id`
- salts remain instance-scoped and therefore do not use workspace RLS
- extension tables reference only `public.workspaces` and `public.installed_extensions`
