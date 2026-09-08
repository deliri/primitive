# v2026.1.19 release gates

User review approved the release checkpoint and explicitly authorized these gates,
the version bump, commit, push, and later disposal of retained test binaries.
The final source passed the requested gates with Go 1.27.1 on Darwin/arm64.
This report records local executions; it does not claim independent acceptance.

| Gate | Scope | Result |
| --- | --- | --- |
| go fix -diff | ./... | Empty diff after applying Go 1.27 fixes |
| go vet | ./... | Pass |
| staticcheck | ./... | Pass |
| deadcode -test | ./... | No unreachable functions |
| witness-lint | All 61 directories discovered by go list ./... | Zero unwaived findings |
| errcheck | ./... | Pass |
| nilaway | ./... | Pass |
| goconst | Production source; minimum length 4, occurrences 3 | Exact four-group admission set |
| go test | ./exchange ./filestore only | Pass, uncached |

The runtime command was `go test -json -count=1 -p=1 -parallel=4 -timeout=20m ./exchange ./filestore`.
Exchange emitted 9,384 passing test events; Filestore emitted 5,515. Neither
package failed or skipped. Parent tests and fuzz seeds count as Go events,
not separately earned semantic cases. Build caching remained enabled.

The [summary](summary.json) names every final record, the source manifest and
all attempts. Failed attempts remain separate: initial Go fix recommendations;
staticcheck and doctrine findings; NilAway diagnostics; archived sources included
by filesystem-recursive tool patterns; two diagnostic edits using the wrong
channel variable; and the second-pass Go fix removal of an obsolete loop binding.
No build or diagnostic mistake is counted as a production regression.

Go fix modernized existing Go syntax. Gate cleanup made comparisons explicit,
preserved compiler-bound inventories, improved failure diagnostics, declared
benchmark allocations at parent entry points, and corrected nil/error guards.
No production execution behavior changed during the gate cleanup. A few existing
tests in Attest, AWSIdentity, Capabilities, Core, ID, Lineio and Timeproof received
those mechanical changes; their runtime tests were not executed in this gate.

Witness retains seven waiver comments, six of which match findings. The new
exceptions preserve the Go-owned integer ceiling and the named result/error that
deferred panic containment requires. The two upstream untyped symlink-loop notes
and three Filestore exceptions remain documented in source.

Goconst finds `benchmark`, `file`, `test`, and `unavailable` in separate closed
domains or provider/CLI contracts. The exact four groups have written reasons in
[scripts/goconst_admissions.tsv](../../scripts/goconst_admissions.tsv); adding or
removing a group fails the ratchet. These are reviewed findings, not zero raw
analyzer findings. No shared constant was invented to couple unrelated meanings.
The canonical gate now applies the requested thresholds and excludes archived
source, hidden scratch directories and tests. Witness receives the actual Go
package directories, so historical snapshots remain evidence rather than lint
subjects.

Previous CPU/memory profiles and benchmark interpretations remain unchanged.
Gate edits to the Exchange benchmark entry points add ReportAllocs before timing
or improve failure diagnostics; they do not establish new performance numbers.
The binary [disposal receipt](binary-cleanup.json) records 212 deleted binaries,
3,171,587,932 file bytes, and every original hash. Earlier artifact audits are
point-in-time checks before this authorized deletion. CPU/memory profiles and
large raw logs remain local; source and small reports are publication candidates.

Linux/Windows runtime acceptance and a fresh race/fuzz/benchmark campaign were
not part of the requested release gates. Prior scope and measurements are in
[the Filestore review](../../filestore/closeout_review.md) and
[the Exchange review](../../exchange/continuation_review.md).
