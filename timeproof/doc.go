// Package timeproof prepares and verifies bounded RFC 3161 timestamp evidence.
//
// The package recognizes a closed set of authorities and trust anchors. It
// delegates transport entirely to callers, entropy to Keygen, and time values
// to Temporal. Only cryptographically verified authority evidence is
// persistable.
package timeproof
