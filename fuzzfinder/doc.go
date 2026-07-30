// Package fuzzfinder finds Go-generated fuzz corpus and crasher artifacts in
// one rooted directory with bounded memory and explicit partial accounting.
//
// Classification is declared by the caller and carried, not inferred. The Go
// toolchain writes cache corpus entries and testdata crashers under the same
// generated-name format, so the containing directory is the only discriminator
// and every observation reports the artifact class its request named.
//
// It does not run fuzz tests, mutate files, retain payload custody, or define a
// consumer's evidence schema.
package fuzzfinder
