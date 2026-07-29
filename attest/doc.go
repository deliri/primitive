// Package attest signs and verifies bounded typed canonical facts with
// Ed25519. It owns canonical-body hashing, domain separation, framing,
// detached envelopes, caller-selected trust, and proof-carrying verification.
//
// Package attest does not own key generation or persistence, body meaning,
// trust-anchor selection, time, storage, transport, or product policy.
//
// A signing domain's canonical text is the whole of the separation between one
// protocol's signatures and another's. That text is the protocol owner's
// namespace: two owners that choose the same text produce interchangeable
// frames, and attest cannot detect it.
package attest
