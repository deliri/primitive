# Execution evidence storage

Store complete Hammer reports, test-run output, benchmark measurements,
mutation results, and release receipts on Furnace under
`/data/evidence/raw/primitive/`. Keep authored `_hammer` claims, Go tests,
semantic fuzz seeds, and test fixtures in source. Generated reports and run
artifacts are not source files and must not be committed.

Historical artifacts removed from revision
`63ce868b74ea0f82052b39ef2cd99ae144824179` are retained at:

`d@192.168.1.81:/data/evidence/raw/primitive/2026-10-08-repository-extraction/`

The archive and its supplement each contain an exact-source manifest,
SHA-256 checksums, and a verifier. Furnace verified every archived file's
byte count and SHA-256 before source removal. Historical document links to
removed artifacts resolve relative to these archives.

This changes artifact storage only. It does not establish new test results
or independent acceptance.
