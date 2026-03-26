# ATS Extension

This is the public first-party `ats` extension for Move Big Rocks, the
AI-native service operations platform.

It is positioned as a real product slice on the shared base, not as a toy
sample or throwaway example.

It is intentionally built from the same primitives customers will use:

- workspace-scoped installation
- seeded queues
- seeded public form that auto-creates candidate cases
- seeded `case_created` automation rule for ATS follow-up tagging
- branded public/admin assets
- declared endpoints, namespaced extension commands, bundled agent-skill assets, and extension events

This extension is intended to be part of the free public first-party bundle
set and is the canonical public bundle source for ATS.

Install it with the operator CLI by pointing at the directory directly:

```bash
mbr extensions install ./ats \
  --workspace WORKSPACE_ID
```

Or install the published public bundle ref:

```bash
mbr extensions install ghcr.io/movebigrocks/mbr-ext-ats:v1.0.0 \
  --workspace WORKSPACE_ID
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
