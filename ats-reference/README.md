# ATS Reference Extension

This is the in-repo reference `ats` extension used to prove that Move Big Rocks, the
AI-native service operations platform, can host workflow-specific products on
shared operational primitives.

It is intentionally built from the same primitives customers will use:

- workspace-scoped installation
- seeded queues
- seeded public form that auto-creates candidate cases
- seeded `case_created` automation rule for ATS follow-up tagging
- branded public/admin assets
- declared endpoints, namespaced extension commands, bundled agent-skill assets, and extension events

Install it with the operator CLI by pointing at the directory directly:

```bash
mbr extensions install ./ats-reference \
  --workspace WORKSPACE_ID \
  --license-token lic_demo_ats
```

The CLI reads `manifest.json` plus every file under `assets/` and uploads the bundle through the same extension install path used for later marketplace bundles.

Inspect the extension-declared agent skills with the generic CLI:

```bash
mbr extensions show --id EXTENSION_ID
mbr extensions skills list --id EXTENSION_ID
mbr extensions skills show --id EXTENSION_ID --name publish-job
```
