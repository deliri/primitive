package core

// Protocol member names that more than one package spells.
//
// A JSON member name is a protocol value: it is part of the bytes a signature
// covers and part of what a decoder matches on. When two packages each write
// the same name as its own literal, nothing in the build notices if one of them
// is later corrected and the other is not, and the two ends of an exchange
// disagree about a fact they both believe they agree on.
//
// Only names that genuinely denote the same fact belong here. A name that one
// package happens to spell the same way while meaning something else is not
// shared vocabulary and must keep its own literal, so this set never becomes a
// bag of coincidentally equal strings.
const (
	// ProtocolMemberAccount names the paying account a fact is scoped to.
	ProtocolMemberAccount = "account"
	// ProtocolMemberOffering names the product an account holds.
	ProtocolMemberOffering = "offering"
)
