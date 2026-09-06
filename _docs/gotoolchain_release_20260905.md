# Primitive v2026.1.14

Approved repair of Hammer pass-2 finding P2-012.

`gotoolchain.AnalyzePackage` now obtains cmd/go metadata through its existing
contained process capability. Cancellation, waiting, and failed-process cleanup
therefore own the compiler subprocesses as well as other Go commands. Parsing,
type checking, and export-data interpretation remain Go compiler operations.
The bounded metadata decoder rejects contradictory import identities rather
than silently overwriting an earlier mapping. Hammer's offline and cache policy
remains in Hammer.

The reviewed candidate is based on
`f33fa94a9c203a7e9aa3090c122a1c455a781bd8`. Local development evidence lives under
`/private/tmp/primitive-review-20260904/`:

- `primitive-pass2-final-race-16`: complete uncached gotoolchain race tests pass.
- `primitive-pass2-fuzz-metadata-17`: 30-second semantic metadata fuzz pass.
- `primitive-pass2-red-process-group` and
  `primitive-pass2-red-import-conflict`: deliberate semantic mutations fail.
- `pass2-review-accounting-45`: Go discovery accounts for all 12 top-level
  tests/fuzz targets, with 90 pass events including subtests and no missing,
  failed, or skipped targets. Discovery/execution source hashes agree.
- Final package Go fix, vet, staticcheck, errcheck, witness-lint, complexity
  at most ten, and goconst (minimum four characters, tests excluded) pass.
  Broad deadcode traversal reports dependency methods, with no gotoolchain
  method reported.

The initial process-group probe could not observe the OS process group inside
the sandbox. That denied attempt remains retained beside the permitted run.
Earlier failures and corrected expectations remain separate execution records.
These local records do not issue independent acceptance.

The public API is unchanged. The release advances compass/config.json from
2026.1.13 to 2026.1.14. Hammer must consume the published tag without a local
replacement and rerun its complete uncached race suite before reporting this
repair delivered in the consumer.
