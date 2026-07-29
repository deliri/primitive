// Package garble owns Primitive's exact Garble tool identity, deterministic
// seed derivation, and the Garble-owned prefix of a typed build intent.
//
// The package does not acquire entropy, persist custody, select release
// identity, execute processes, or assemble complete Go build commands. Those
// responsibilities remain with their owning composition packages.
package garble
