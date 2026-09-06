# Primitive v2026.1.15

The Go compiler metadata loader now follows cmd/go's treatment of the special
`C` import: it does not demand an ordinary dependency-package record for it.
Missing ordinary imports still fail with `core.ErrGoToolchainOutput`.
The implementation is pure Go and adds no cgo dependency. Parsing and type
checking continue through Go's standard compiler, parser, importer, and types.

Release validation also restores five missing compile-time JSON validation
witnesses and removes two obsolete filesystem-call allowances from the core
ownership ratchet. Both issues reproduced on untouched v2026.1.14 before repair.

## Validation

- The metadata regression and an actual cgo compiler fixture failed before the
  fix and passed afterward. The fixture's C import is test input, not a runtime
  dependency of Primitive.
- The full uncached race suite passed outside `core`; the two pre-existing core
  ratchet failures were then repaired. Uncached race tests passed for `core`,
  `gotoolchain`, `capabilities`, `sourceobservation`, and `googleidentity` after
  those repairs.
- Compiler metadata semantic fuzzing passed for 30 seconds (183,225 executions).
- Vet, staticcheck, errcheck, witness-lint, and production complexity checks
  passed for the compiler change; the repaired witness packages passed vet,
  errcheck, and witness-lint.
- A private Blink Kernel copy built all four deployment targets for
  `GOOS=linux GOARCH=amd64 CGO_ENABLED=0` against this patch. Their selected
  dependency graphs contained no cgo files and no `gotoolchain` package.

Local evidence is retained under `/private/tmp/hammer-authored-4taGJd`.
