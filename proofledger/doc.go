// Package proofledger defines the blind append-only agreement shared by proof
// producers, durable providers, and readers. It knows canonical bytes, chain
// integrity, bounded pagination, idempotency, and receipts; payload meaning is
// supplied by the typed domain using it.
//
// Its receipt is specifically an AppendReceipt: it repeats the durable event
// coordinates and has no second receipt identity. Attest alone authenticates
// that result. Proofledger alone calculates and verifies predecessor linkage.
package proofledger
