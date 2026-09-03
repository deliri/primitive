// Package runprotocol defines the shared agreement between an independent run
// requester and an execution authority.
//
// It owns bounded request, admission, machine, execution-accounting,
// observation, artifact, and receipt-shaped facts. It does not inspect source,
// explain why code exists, execute a runner, retain history, or decide whether
// evidence satisfies product policy. Source claims, source observations, and
// their proofs belong to sourceclaim, sourceobservation, and sourceproof.
package runprotocol
