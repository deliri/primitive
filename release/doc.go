// Package release verifies exact build tools, constructs deterministic
// fixed-target Garble build and process plans, inspects the resulting native
// executables, binds signed tool and metadata provenance into immutable
// manifests, authenticates Latest selections, then makes the
// installed-versus-latest decision.
//
// Release owns no files, transport, download, installation, scheduling,
// persistence, retry, or customer-facing policy.
package release
