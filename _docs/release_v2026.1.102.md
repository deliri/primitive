# Primitive v2026.1.102

Hostfacts now provides `ObserveTimeZone(TimeZoneRequest)` for one explicit
IANA timezone lookup through Go's `time.LoadLocation`. It returns Go's actual
location and preserves a typed Core contract or observation error with the
Hostfacts operation identity. Empty names, server-local inference, malformed
paths and overlong names cannot silently choose the customer's timezone.

`core.TimeZoneNameMaximumBytes` is the shared request ceiling. Error identities
remain Core-owned constants (`ErrHostFactsContract`, `ErrHostFactsObservation`).
Applications continue to own timezone selection, birthday windows, calendar
periods, caching and product decisions. No zone database, product state machine,
retry loop or independent clock implementation was added.

Verification includes all 67 public packages with Go test cache reuse disabled,
race tests for Hostfacts/Core/Capabilities, semantic timezone fuzzing, and two
discarded mutations: admitting `Local` and replacing the requested zone with
UTC. Both mutations fail the behavioral test. The full run records three skips
and seven deliberate failing child processes inside passing isolation tests.
An earlier parallel full run failed an architecture inventory and two test I/O
collection attempts; those attempts remain in the evidence.

Final checks include repository-wide go vet and vulnerability/dead-code scans,
plus affected-package staticcheck, nilaway, errcheck, witness-lint, gosec,
complexity, go fix and Hostfacts field alignment. The repository-wide alignment
gate still reports unmodified Passwordhash, Plunk and Exchange layouts; this
release does not claim that full gate is clean. No performance change is claimed
and benchmarks were not run for this mechanical lookup.

Retained command vectors, source digests, attempts, stdout/stderr and mutation
overlays are in Cleanlift's
`_docs/work/2026-10-03/primitive-102/`. Release verification is implementation
evidence. Independent review and acceptance remain with the user.
