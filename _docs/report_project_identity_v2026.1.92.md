# Authority-issued project identity

ReportScope.Project now uses Primitive's existing id.ULID directly. The first
report agreement incorrectly admitted repository-style slugs despite the API
design requiring globally unique server-issued project identities. The slug type,
decoder and compatibility surface are removed. Company/offering/project/device/
epoch remain jointly signed and verified. Both consumers must update together.

TestReportScopeUsesAuthorityIssuedProjectIdentity failed against production
c3d1e92daa7b75eca39903ed02b20fd85b20bb63 with its added structural test, then passed
after the type change. The permit suite passed 209 test events with no failure
or skip. Signed-document, authorization and domain fuzz phases, vet and doctrine
lint were rerun. Local attempts and artifact digests are retained in the adjacent
JSON; this is not independent acceptance. Prior v2026.1.91 findings and the known
core GCS ownership-inventory mismatch remain recorded in its release note.

The existing id package owns ULID representation fuzzing; permit's signed fuzz
oracles exercise that decoder inside every project-bound signed carrier. No
project slug decoder remains to preserve as compatibility ballast.
