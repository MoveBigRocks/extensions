# Enterprise Access

The `enterprise-access` extension is the first-party enterprise SSO extension
for Move Big Rocks, the AI-native service operations platform.

Its job is to let your Move Big Rocks instance authenticate against your company's existing IdP
without turning Move Big Rocks into an identity product or moving ownership of users, sessions,
memberships, or break-glass admin access out of core.

Scope:

- OIDC-first provider contract
- instance-scoped installation
- privileged runtime preflight
- admin settings route with provider persistence in the extension-owned schema
- OIDC authorization start flow
- OIDC callback flow with session creation
- runtime health endpoint with provider-aware status
- agent skill for guided setup

This extension is intentionally stricter than product extensions such as ATS or analytics:

- `kind = identity`
- `scope = instance`
- `risk = privileged`
- trusted first-party publisher only
- no public pages or public assets
- health endpoint required

The runtime and CLI install, inspect, configure, and activate this extension.
Provider configuration lives in the extension-owned PostgreSQL schema.

Canonical schema migrations for the owned `ext_*` schema live under:

- `migrations/000001_init.up.sql`

The pre-production baseline is already folded into `000001_init.up.sql`,
including provider `user_info_url` support.

Those versions are tracked in `core_extension_runtime.schema_migration_history`,
not in `public.schema_migrations`.

## Hardening Rules

Treat this extension as a privileged boundary:

- only configure trusted enterprise IdPs
- use HTTPS for discovery, authorization, token, userinfo, and JWKS endpoints
- keep client secrets in a secret manager or deployment-secret path and reference them through `clientSecretRef`
- review claim mapping and role mapping before activation
- dogfood it in a sandbox workspace or controlled rollout path before enforcing SSO broadly

Do not use arbitrary or unreviewed provider URLs in production.

Production policy is intentionally stricter:

- `clientSecretRef` must use an explicit scheme such as `env:NAME`
- raw literal client secrets are rejected
- `env:` secret refs should be considered a development or break-glass option and can be disabled with `ENTERPRISE_ACCESS_ALLOW_ENV_SECRET_REFS=false`
- set `ENTERPRISE_ACCESS_ALLOWED_HOSTS` to the reviewed IdP issuer and endpoint hostnames you want this extension to trust

The OIDC callback endpoint is public by transport, but it is still protected by
signed, time-bounded `state` validation and safe post-login return-path
sanitization. It is not a generic inbound webhook endpoint.

Repository status:

- first-party extension source lives in `MoveBigRocks/extensions`
- not part of the public OCI bundle catalog
