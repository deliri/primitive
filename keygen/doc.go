// Package keygen constructs exact Ed25519 signing keys and bounded generic
// secret material with Go's production cryptographic random APIs.
//
// It does not export entropy providers, derive product keys, select algorithms
// at runtime, persist material, manage rotation, or own product policy.
package keygen
