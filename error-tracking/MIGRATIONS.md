# Error Tracking Migration Boundary

Owned schema:

- `ext_demandops_error_tracking`

Canonical migration files:

- `migrations/000001_init.up.sql`
- `migrations/000002_rls.up.sql`

What belongs in these migrations:

- projects/applications
- issues
- error events
- alerts
- project stats
- issue stats
- git repository mappings used by issue resolution flows
- indexes and RLS policies for the owned schema

What does not belong in these migrations:

- public ingest route mounting
- admin page registration
- support-case synchronization logic beyond stored raw IDs
- outbox infrastructure
- one-off data import/export logic

Schema rules:

- all workspace-scoped tables carry both `workspace_id` and `extension_install_id`
- direct foreign keys stay within the extension schema, plus allowed references
  to `public.workspaces` and `public.installed_extensions`
- git repository mappings move with the product because they are keyed by the
  error-tracking project/application identity
