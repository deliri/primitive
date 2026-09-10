// Package fuzzartifact finds Go-generated fuzz corpus and crasher names in one
// rooted directory. Find streams every match through a synchronous visitor in
// native directory order, with fixed working memory and no retained-name quota.
// The caller owns sorting, selection, payload reads, and visitor effects.
// Directory mutation during a scan follows Go and operating-system semantics;
// enumeration is not a snapshot.
//
// Classification is declared by the caller and carried, not inferred. The Go
// toolchain writes cache corpus entries and testdata crashers under the same
// generated-name format, so names alone cannot distinguish their purpose.
// Observations report the requested class and exact or saturated entry counts.
// A visited name proves neither the contents nor continued existence of a file.
//
// It does not discover FuzzXxx functions, run fuzz tests, mutate files, retain payload custody, or define a
// consumer's evidence schema.
package fuzzartifact
