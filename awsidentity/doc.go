// Package awsidentity acquires one bounded opaque identity bearer from the
// regional AWS STS GetWebIdentityToken protocol.
//
// The caller owns SigV4 credential discovery and signing. Awsidentity admits
// the exact signed capability, performs one bounded request through Exchange,
// validates the provider's XML envelope, and returns a redacted token. It does
// not cache, refresh, verify claims, or grant product authority.
package awsidentity
