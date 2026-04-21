# Manage Web-Analytics Properties

Use this skill when an operator or agent needs to register, list, update, or
delete analytics properties (tracked domains) without going through the
browser admin UI.

## When to use

- A new site needs tracking and the tracking script expects a `data-domain`
  that matches a registered property.
- An existing property needs its timezone corrected, or should be paused.
- A site has been decommissioned and its property should be removed.

## Endpoints

All endpoints sit at `/extensions/web-analytics/api/agent/properties` with
`auth: agent_token` and `workspaceBinding: workspace_from_agent_token`. They
share business logic with the session-authed admin endpoints (same service
targets), so anything done via the agent surface is immediately visible in the
admin UI and vice versa.

| Method | Path                                                | Service target                      |
|--------|-----------------------------------------------------|-------------------------------------|
| GET    | `/extensions/web-analytics/api/agent/properties`    | `analytics.api.properties`          |
| POST   | `/extensions/web-analytics/api/agent/properties`    | `analytics.api.properties.create`   |
| GET    | `/extensions/web-analytics/api/agent/properties/:id`| `analytics.api.property.get`        |
| PATCH  | `/extensions/web-analytics/api/agent/properties/:id`| `analytics.api.property.update`     |
| DELETE | `/extensions/web-analytics/api/agent/properties/:id`| `analytics.api.property.delete`     |

## Minting an agent token

The calling agent needs a token scoped to the workspace where the
web-analytics extension is installed:

```bash
mbr agents tokens create <AGENT_ID> --name "web-analytics-properties" --expires-in-days 30
```

Agent membership on the workspace is required. No additional named permission
is gated on these endpoints in the first release; the workspace binding
enforces scope.

## Register a new property

```bash
curl -sS -X POST "$MBR_URL/extensions/web-analytics/api/agent/properties" \
  -H "Authorization: Bearer $MBR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "domain": "example.com",
    "timezone": "Europe/Amsterdam"
  }'
```

On success (`201 Created`) the response body is:

```json
{
  "property": {
    "id": "019d...",
    "domain": "example.com",
    "timezone": "Europe/Amsterdam",
    "status": "active",
    ...
  }
}
```

Immediately afterwards, the analytics.js snippet on that domain will start
being accepted:

```html
<script defer data-domain="example.com" src="https://<mbr-host>/js/analytics.js"></script>
```

## List, get, update, delete

```bash
# list
curl -sS -H "Authorization: Bearer $MBR_TOKEN" \
  "$MBR_URL/extensions/web-analytics/api/agent/properties"

# get
curl -sS -H "Authorization: Bearer $MBR_TOKEN" \
  "$MBR_URL/extensions/web-analytics/api/agent/properties/$PROPERTY_ID"

# update timezone or status (partial update)
curl -sS -X PATCH -H "Authorization: Bearer $MBR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"timezone":"Europe/Paris"}' \
  "$MBR_URL/extensions/web-analytics/api/agent/properties/$PROPERTY_ID"

# delete
curl -sS -X DELETE -H "Authorization: Bearer $MBR_TOKEN" \
  "$MBR_URL/extensions/web-analytics/api/agent/properties/$PROPERTY_ID"
```

## Confirmation discipline

- Before `DELETE`, confirm the property id and the fact that events for that
  domain will stop being recorded. Deletions cascade to sessions, events, and
  goals.
- Before changing `domain`, confirm the tracking snippet on the site will be
  updated to match. A mismatch causes silent data loss.
- Do not register the same domain twice. The `domain` column has a
  case-insensitive unique index; the create call will return `400` if the
  domain already exists in the workspace.
