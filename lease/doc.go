// Package lease verifies and assesses one fixed-size, authority-signed lease
// decision.
//
// The issuer owns accounts, plans, payment standing, trials, device limits,
// policy, and the choice to issue a grant, refusal, or revocation. Package
// lease owns only the signed decision's typed shape, subject binding, exact
// timeline, monotonic generation advance, and pure local assessment.
//
// The package performs no clock read, persistence, transport, retry,
// background work, command authorization, or product-specific rendering.
// Consumers supply real temporal observations and durably commit selected
// decisions and returned high-water instants before creating paid work.
package lease
