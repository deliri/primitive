// Package googleidentity acquires bounded outbound identity and OAuth access
// bearers from Google Cloud metadata.
//
// Each token is opaque and redacted. Googleidentity can also validate the exact
// bounded stdout of a caller-owned Google Cloud credential command, but it
// does not discover credentials, execute provider tools, cache or refresh
// tokens, or authorize consumer operations. Access-token acquisition returns
// the provider-declared positive lifetime but owns no refresh policy. Receivers
// own signature, issuer, audience, expiry, principal, scope, and authorization
// decisions.
package googleidentity
