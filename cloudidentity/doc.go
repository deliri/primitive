// Package cloudidentity acquires one bounded outbound identity-token bearer
// from Google Cloud metadata or regional AWS STS.
//
// The token is opaque and redacted. Cloudidentity can also validate the exact
// bounded stdout of a caller-owned Google Cloud credential command, but it
// does not discover credentials, execute provider tools, cache or refresh
// tokens, or authorize consumer operations. Receivers own signature, issuer,
// audience, expiry, and principal verification.
package cloudidentity
