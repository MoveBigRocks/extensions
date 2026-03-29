# Enterprise Access Migrations

The `enterprise-access` extension owns an instance-scoped PostgreSQL schema.

The migrations create and evolve the provider and provisioning-rule tables needed to store
enterprise identity configuration without moving core users, sessions, or memberships out of
the main Move Big Rocks schema.

Current canonical baseline:

- `migrations/000001_init.up.sql`

Because Move Big Rocks is still pre-production, the current baseline already includes
the final provider shape, including `user_info_url`. Future schema changes
should append new migration versions instead of reintroducing transitional
follow-up files for fields that belong in the initial owned schema.

Design rules:

- core still owns users, sessions, memberships, and authorization
- this schema stores provider configuration and provisioning rules
- provider metadata is persisted in `${SCHEMA_NAME}.identity_providers`, not in the generic extension config blob
- workspace linkage is explicit through foreign keys where needed
- uninstall should archive/export first and only drop schema when the operator chooses to
