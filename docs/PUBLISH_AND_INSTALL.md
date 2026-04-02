# Publish And Install First-Party Extensions

This runbook turns the source in this repo into real public GHCR packages and
then installs those packages into the DemandOps Move Big Rocks instance.

## What Actually Creates The Packages

GitHub Packages does not populate itself from folders in this repo.

The packages are created only when
[`../.github/workflows/public-bundles.yml`](../.github/workflows/public-bundles.yml)
runs successfully for one of these release tags:

- `ats-v<version>`
- `community-feature-requests-v<version>`
- `error-tracking-v<version>`
- `sales-pipeline-v<version>`
- `web-analytics-v<version>`

That workflow:

- builds the bundle from the extension directory
- checks out the pinned `MoveBigRocks/extension-sdk` release used for tooling
- signs it with `MBR_EXTENSION_SIGNING_PRIVATE_KEY_B64`
- publishes it to GHCR
- uploads the signed bundle and publisher-key snippet as artifacts

The current pinned SDK tooling ref is `MoveBigRocks/extension-sdk@v0.8.22`.
When the SDK tooling changes, cut a new SDK tag first and then update the
workflow pin in this repo.

Before tagging and publishing, each extension directory should pass:

```bash
mbr extensions lint ./EXTENSION_DIR --json
mbr extensions verify ./EXTENSION_DIR --workspace WORKSPACE_ID --json
mbr extensions nav --instance --json
mbr extensions widgets --instance --json
```

If the declared extension surface changed intentionally, refresh the checked-in
contract file first:

```bash
mbr extensions lint ./EXTENSION_DIR --write-contract --json
```

Do not treat the workspace-scoped happy path as sufficient proof on its own.
For any extension with admin UI, also confirm that an instance admin with no
active workspace selection can still discover and open the extension cleanly.

For the public first-party catalog, the repo-level proof loop is:

```bash
MBR_BIN=/path/to/mbr bash ./scripts/validate-first-party.sh
```

## Prerequisites

Before the first publish, make sure the `MoveBigRocks/extensions` repo has:

- the GitHub Actions secret `MBR_EXTENSION_SIGNING_PRIVATE_KEY_B64`
- package publish permissions enabled for Actions
- the repo public, so the source and package lineage are public

The workflow publishes these package names:

- `ghcr.io/movebigrocks/mbr-ext-ats`
- `ghcr.io/movebigrocks/mbr-ext-community-feature-requests`
- `ghcr.io/movebigrocks/mbr-ext-error-tracking`
- `ghcr.io/movebigrocks/mbr-ext-sales-pipeline`
- `ghcr.io/movebigrocks/mbr-ext-web-analytics`

Generate the signing seed and trusted publisher JSON once from the SDK:

```bash
go run ./scripts/generate-signing-key.go \
  --publisher DemandOps \
  --key-id demandops-public-1 \
  --seed-out secrets/demandops-public-1.seed.b64 \
  --trusted-publishers-out dist/demandops-public-1.publisher.json
```

Then:

- put the seed file content into the GitHub Actions secret `MBR_EXTENSION_SIGNING_PRIVATE_KEY_B64`
- add the trusted publisher JSON to the instance config as `EXTENSION_TRUSTED_PUBLISHERS_JSON`
- if the publish workflow fails in `Sign public bundle`, first verify that the secret exists and contains the raw base64 seed or private key with no extra quoting

## First Publish

From a checkout of this repo, cut tags that match the manifest versions in the
extension directories. For the current ATS source that means `ats-v0.8.29`:

```bash
git tag ats-v0.8.29
git tag community-feature-requests-v0.1.0
git tag error-tracking-v0.8.21
git tag sales-pipeline-v0.1.0
git tag web-analytics-v0.8.21
git push origin ats-v0.8.29 community-feature-requests-v0.1.0 error-tracking-v0.8.21 sales-pipeline-v0.1.0 web-analytics-v0.8.21
```

That should trigger five workflow runs and create five GHCR packages.

## After The First Publish

For each package, open GitHub Packages and set visibility to `Public`:

- `mbr-ext-ats`
- `mbr-ext-community-feature-requests`
- `mbr-ext-error-tracking`
- `mbr-ext-sales-pipeline`
- `mbr-ext-web-analytics`

Then verify that the install refs you expect to use are the real published
ones:

- `ghcr.io/movebigrocks/mbr-ext-ats:v0.8.29`
- `ghcr.io/movebigrocks/mbr-ext-community-feature-requests:v0.1.0`
- `ghcr.io/movebigrocks/mbr-ext-error-tracking:v0.8.21`
- `ghcr.io/movebigrocks/mbr-ext-sales-pipeline:v0.1.0`
- `ghcr.io/movebigrocks/mbr-ext-web-analytics:v0.8.21`

## Install Into DemandOps

The DemandOps instance repo already records the intended refs in
`mbr-prod/extensions/desired-state.yaml`.

Authenticate to the live instance first:

```bash
mbr auth login --url https://mbr.demandops.com
```

Resolve the live workspace IDs:

```bash
mbr workspaces list --url https://mbr.demandops.com
```

The current DemandOps desired-state mapping is:

- ATS -> `people`
- web analytics -> `marketing`
- error tracking -> `engineering`

Install, validate, and activate with the real workspace IDs:

```bash
mbr extensions install ghcr.io/movebigrocks/mbr-ext-ats:v0.8.29 --url https://mbr.demandops.com --workspace WORKSPACE_ID_FOR_PEOPLE --json
mbr extensions validate --url https://mbr.demandops.com --id EXTENSION_ID
mbr extensions activate --url https://mbr.demandops.com --id EXTENSION_ID
```

```bash
mbr extensions install ghcr.io/movebigrocks/mbr-ext-web-analytics:v0.8.21 --url https://mbr.demandops.com --workspace WORKSPACE_ID_FOR_MARKETING --json
mbr extensions validate --url https://mbr.demandops.com --id EXTENSION_ID
mbr extensions activate --url https://mbr.demandops.com --id EXTENSION_ID
```

```bash
mbr extensions install ghcr.io/movebigrocks/mbr-ext-error-tracking:v0.8.21 --url https://mbr.demandops.com --workspace WORKSPACE_ID_FOR_ENGINEERING --json
mbr extensions validate --url https://mbr.demandops.com --id EXTENSION_ID
mbr extensions activate --url https://mbr.demandops.com --id EXTENSION_ID
```

Public signed bundles do not need `--license-token`.

## ATS Dedicated Workspace Option

The ATS manifest supports `workspacePlan.mode = provision_dedicated_workspace`.
If DemandOps wants ATS in a dedicated `hiring` workspace instead of the
existing `people` workspace, install ATS without `--workspace` while using
browser-backed session auth, then update the DemandOps desired-state file to
match that decision.
