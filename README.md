# Move Big Rocks Extension Examples

This repo is the public home for non-sensitive example extensions for Move Big
Rocks.

## Included Examples

- `ats-reference`
  A real installable reference extension that demonstrates the extension
  lifecycle, workspace-scoped installation, seeded ATS workflows, branded
  assets, and bundled agent skills.

## Using The ATS Reference Example

Install from a checked-out source directory:

```bash
mbr extensions install ./ats-reference \
  --workspace WORKSPACE_ID \
  --license-token lic_demo_ats
mbr extensions validate --id EXTENSION_ID
mbr extensions activate --id EXTENSION_ID
```

Or from a published bundle artifact:

```bash
mbr extensions install ghcr.io/movebigrocks/ats-reference:v1.0.0 \
  --workspace WORKSPACE_ID \
  --license-token lic_demo_ats
```

## Repo Rules

- keep examples installable from source checkout
- keep examples non-sensitive and non-privileged
- do not use this repo as the source of truth for a commercial or privileged pack
