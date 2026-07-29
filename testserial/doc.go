// Package testserial declares why a Go test must remain non-parallel.
//
// Declare validates a compiler-owned isolation contract for the pinned source
// analyzer. It does not acquire a lock, schedule tests, or provide mutual
// exclusion.
package testserial
