// Package loops owns Loops credentials, closed operation routes, authenticated
// HTTP exchange, and verify-before-release Standard Webhooks receipt without
// interpreting email semantics. Rate-limit (429) and idempotency-conflict
// (409) replies are exact Exchange results; the caller owns any backoff.
package loops
