package atsmigrations

import "embed"

// Files exposes the ATS owned-schema migration set for runtime proofs and tests.
//
//go:embed *.sql
var Files embed.FS
