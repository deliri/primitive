// Package projectversion defines the compiler-owned release identity shared by
// Primitive-based projects.
//
// A project owns its release coordinates as three constants in its core
// package. It passes those constants to New and uses the returned Release for
// build manifests, update comparisons, display text, and Git tags. Projects
// must not keep a second string version, spell a "v" prefix themselves, or
// parse their own release identity back from text.
package projectversion
