# Publish And Install First-Party Extensions

This runbook covers the public bundles in
[the catalog](../catalog/public-bundles.json). Customer deployment intent lives
in each private instance repository; this document does not record live
DemandOps workspace assignments or installed versions.

## Sources of version truth

| Input | Source |
| --- | --- |
| Extension source version and endpoints | Each extension's `manifest.json` |
| Expected extension surface | Each `extension.contract.json` |
| Public bundle/runtime repository names and Git tag patterns | `catalog/public-bundles.json` |
| Bundle build/sign tooling version | `SDK_REF` in [public-bundles.yml](../.github/workflows/public-bundles.yml) |
| Runtime Go SDK dependency | `go.mod` and `vendor/modules.txt` |
| Desired installed version | The target instance's `extensions/desired-state.yaml` |
| Accepted published bytes | Successful workflow output and verified registry digest |

The tooling pin and runtime SDK dependency are separate inputs. Read them from
source instead of copying a version number from an old runbook:

```bash
rg 'SDK_REF:' .github/workflows/public-bundles.yml
rg 'github.com/movebigrocks/extension-sdk ' go.mod
jq -r '.version' ats/manifest.json
```

## Validate before publication

From the repository root, using the intended platform CLI:

```bash
bash scripts/check-public-boundary.sh
MBR_BIN=/path/to/mbr bash scripts/validate-first-party.sh
GOWORK=off go test -race ./...
GOWORK=off go build ./cmd/...
```

Set `TEST_DATABASE_ADMIN_DSN` to a disposable PostgreSQL 18 administrator
connection before running tests. The database helpers can skip when it is
absent; skipped database tests are not evidence of a successful integration run.
The Go version/toolchain is specified in `go.mod`. The catalog regression test
checks that every source manifest is discoverable and that its published catalog
classification, including installation scope, agrees with the manifest.

The validation script runs local `extensions lint` across all discovered source
packages. It does not install, activate, or prove a live product workflow.
Run those checks against an isolated test instance as well:

```bash
mbr extensions verify ./EXTENSION_DIR --workspace WORKSPACE_ID --json
mbr extensions nav --instance --json
mbr extensions widgets --instance --json
```

Verify both workspace navigation and an instance administrator with no selected
workspace. If the declared surface changes intentionally, review and refresh
its contract with `mbr extensions lint ./EXTENSION_DIR --write-contract --json`.
Do not refresh a contract merely to suppress an unexpected difference.

## Signing prerequisites

Configure the Actions secret `MBR_EXTENSION_SIGNING_PRIVATE_KEY_B64` and package
write permissions. Generate a signing seed and publisher snippet from a checkout
of the SDK tooling version selected by the publication workflow:

```bash
go run ./scripts/generate-signing-key \
  --publisher DemandOps \
  --key-id demandops-public-1 \
  --seed-out secrets/demandops-public-1.seed.b64 \
  --trusted-publishers-out dist/demandops-public-1.publisher.json
```

Keep the seed out of Git. Put its raw base64 value into the Actions secret and
configure the public publisher snippet as `EXTENSION_TRUSTED_PUBLISHERS_JSON`
on the target instance. Signature verification checks origin and integrity;
operators still review runtime permissions, behavior and required credentials.

## Publish a selected extension

Choose the extension deliberately and ensure the working tree is clean, its
changes are tested, and its source version is the intended release. For ATS:

```bash
extension=ats
version="$(jq -r '.version' "$extension/manifest.json")"
bash scripts/report-first-party-release-state.sh
git tag "${extension}-v${version}"
git push origin "${extension}-v${version}"
```

Run the tag/push steps only when the report and remote tag inventory confirm that
this is a new intended release. Do not move or reuse an existing release tag.

The [publication workflow](../.github/workflows/public-bundles.yml):

1. Builds the Linux amd64 runtime executable from the extension repository.
2. Publishes its archive to `mbr-ext-<slug>-runtime:v<version>` and resolves the digest.
3. Copies bundle source into a staging directory and injects the runtime ref,
   digest and release version into the staged manifest.
4. Uses the pinned SDK tooling to build and sign the bundle.
5. Publishes `mbr-ext-<slug>:v<version>` and records bundle/runtime digests.
6. Uploads signed bundle and publisher-key artifacts.

Source manifests use `sha256:pending` before staging. They are development input,
not proof of a deployable runtime digest. The workflow stamps the version from
the release tag; keeping the source version aligned is an operator release rule,
not a claim that the workflow rejects every mismatch.

Manual dispatch is for preview tags such as `sha-<commit>`. The workflow rejects
semver-like manual tags and manual `latest` promotion. Git release tags publish
versioned refs and also update `latest`; production instances should pin reviewed
versions/digests rather than follow `latest`.

## Verify registry publication

After the first publication, ensure each intended public package is anonymously
pullable. Inspect package visibility and the successful workflow summary; an
empty Packages tab alone does not explain whether publication failed or access
is restricted. Verify both bundle and runtime digests before deploying.

A Git tag or manifest version alone does not prove a registry artifact exists.
`report-first-party-release-state.sh` reports source/tag state, not signature,
runtime startup or installed-instance acceptance.

## Install and reconcile

The production control plane is the private instance repository. Record verified
refs, scope, workspace and configuration in `extensions/desired-state.yaml`, run
that repo's validator, then use its deploy/reconcile workflow. Runtime binaries
must be staged where the host supervisor expects them before activation.

For an explicit development or repair install, authenticate to the target host
and resolve the real workspace ID:

```bash
mbr auth login --url https://admin.example.com
mbr workspaces list --url https://admin.example.com
mbr extensions install ghcr.io/movebigrocks/mbr-ext-ats:v<VERSION> \
  --url https://admin.example.com --workspace WORKSPACE_ID --json
mbr extensions validate --url https://admin.example.com --id EXTENSION_ID
mbr extensions activate --url https://admin.example.com --id EXTENSION_ID
mbr extensions monitor --url https://admin.example.com --id EXTENSION_ID
```

Substitute the selected, verified version and use the base URL required by the
instance's CLI routing. Public signed bundles do not require `--license-token`;
controlled instance-bound distribution can require it. Reflect manual changes
in desired state so the next reconciliation does not undo them.

## Dedicated workspaces and preview scope

ATS, sales pipeline and community feature requests declare dedicated-workspace
plans. Omitting `--workspace` with browser-backed administrator authentication
allows the host to apply that plan. Passing a workspace explicitly selects that
workspace; it does not imply provisioning a separate one.

Use an isolated instance for untrusted code or incompatible runtime experiments.
A preview workspace scopes data; it does not isolate processes, shared runtimes
or public route namespaces. Public routes can collide across workspace installs.
