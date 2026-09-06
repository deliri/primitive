# Primitive v2026.1.16

Compiler analysis now accepts the empty production package that cmd/go emits
for a directory containing only test files. The package identity comes from
cmd/go; Go's standard `types.NewChecker` checks its selected syntax. Included
tests remain analyzed, and invalid included tests still produce a typed
compiler-output refusal. This adds no dependency or cgo requirement.

The regression failed against v2026.1.15 before the change. The uncached
gotoolchain race suite, vet, staticcheck, errcheck, witness-lint, and the
production complexity check passed after the change. Evidence is retained in
`/private/tmp/hammer-authored-4taGJd/primitive-test-only-*`.
