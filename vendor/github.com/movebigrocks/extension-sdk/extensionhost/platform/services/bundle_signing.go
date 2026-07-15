package platformservices

import "encoding/json"

// The bundle signer and this verifier must canonicalize a signed bundle the
// exact same way, or a bundle this package signs is one it would reject. They
// drifted once: the signer carried its own copy of the license-claim shape with
// every field marked omitempty, while the verifier kept instanceID, slug,
// version, and tokenSHA256 non-omitempty, so a public bundle serialized its
// license differently on each side and every signature failed verification.
//
// These exports give the signer the one canonicalization and the one
// license-claim shape defined here, so a signer built on them cannot drift from
// VerifyBundle. check-core-security-parity.sh additionally gates this file's
// canonicalization and license-claim regions against the platform source, so
// the SDK copy cannot drift from the core verifier either.

// BundleLicenseClaim is the license shape a publisher signs over and the
// verifier reconstructs. It is an alias, not a new type, so the signer and
// verifier share one definition.
type BundleLicenseClaim = bundleLicenseClaim

// CanonicalSignedBundlePayload returns the canonical bytes a publisher signs
// and VerifyBundle checks. Sign the output with the publisher key; store the
// signature and this license in the bundle trust envelope.
func CanonicalSignedBundlePayload(manifestRaw, assetsRaw, migrationsRaw json.RawMessage, license BundleLicenseClaim) ([]byte, error) {
	return canonicalSignedBundlePayload(manifestRaw, assetsRaw, migrationsRaw, license)
}

// ChecksumSHA256Hex returns the lowercase hex SHA-256 of value, matching the
// token hashing the verifier uses for instance-bound bundles.
func ChecksumSHA256Hex(value []byte) string {
	return checksumSHA256Hex(value)
}
