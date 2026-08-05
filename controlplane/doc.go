// Package controlplane owns the signed documents a product control plane and
// its installed clients exchange.
//
// Both ends of the exchange run this same package, so there is no published
// contract for one side to mirror and no pair of copies that can drift. An
// authority issues a document and an installation verifies it against the same
// types, the same bounds, and the same canonical bytes.
//
// The package owns the documents and the rules that make one document belong to
// one exact request: the response header and its binding to the request that
// produced it, the registration request an installation sends, the signed
// registration response it receives, the commercial status that response
// carries, and the usage watermark that orders one installation's accepted
// windows.
//
// It does not own the decisions. Which installations are entitled, what a
// policy revision permits, how an account is billed, and which key signs are
// the authority's business, and nothing here reads them. Controlwire owns the
// scalars underneath these documents, Lease owns the timeline a grant creates,
// Attest owns the envelope, and this package composes them.
//
// Nothing here reaches the network, the clock, or the disk.
package controlplane
