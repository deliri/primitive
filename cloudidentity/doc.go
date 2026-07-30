// Package cloudidentity acquires one bounded outbound identity-token bearer
// from Google Cloud metadata or regional AWS STS.
//
// The token is opaque and redacted. Cloudidentity does not verify token
// claims, discover credentials, sign AWS requests, cache or refresh tokens,
// execute provider tools, or authorize consumer operations. Receivers own
// signature, issuer, audience, expiry, and principal verification.
package cloudidentity
