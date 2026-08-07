// Package keygen constructs exact Ed25519 signing keys, bounded generic secret
// material, and bounded public random tokens with Go's production
// cryptographic random APIs.
//
// It is the entropy boundary: every bounded random draw a consumer needs is
// made here, so no consumer reaches into crypto/rand for a nonce, a seed, or a
// device label. It does not export a streaming entropy provider, derive
// product keys, select algorithms at runtime, persist material, manage
// rotation, or own product policy.
package keygen
