// Package release authenticates immutable release manifests and Latest
// selections, then makes the pure installed-versus-latest decision.
//
// Release owns no files, transport, download, installation, scheduling,
// persistence, retry, or customer-facing policy.
package release
