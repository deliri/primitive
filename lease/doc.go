// Package lease verifies and assesses one fixed-size, authority-signed lease
// decision.
//
// The issuer owns accounts, plans, payment standing, trials, device limits,
// policy, and the choice to issue a grant, refusal, or revocation. Package
// lease owns only the signed decision's typed shape, subject binding, exact
// timeline, monotonic generation advance, and pure local assessment.
//
// JSON byte-slice entry points take caller-owned complete values. Their input
// allocation follows the supplied representation, including whitespace; they
// impose no transfer-size quota. Canonical output is a fixed-size typed decision.
// These whole-value APIs do not claim constant-memory streaming of JSON input.
//
// The package performs no clock read, persistence, transport, retry,
// background work, command authorization, or product-specific rendering.
// Consumers supply real temporal observations and durably commit selected
// decisions and returned high-water instants before creating paid work.
package lease
