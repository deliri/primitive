# Primitive v2026.1.21

This checkpoint integrates the compiler-analysis and filesystem additions from
`/private/tmp/hammer-finish-o5b91ph5/primitive-integration-172`, together with the
pending Process, Core and Contextstate work, as approved by the user. Package
hardening remains an incremental program; this release does not claim every
package has completed the testing-protocol sweep.

GoToolchain supports bounded batches, per-package partial analysis, original
Go parser/type-checker diagnostics and compiler-selected cgo source. Metadata
streams through Go readers and process execution retains Primitive containment.
Independent checks retain separate Go checker/importer ownership. Compiler
analysis necessarily holds the selected syntax and type graph; it does not
claim constant memory for a complete compiler result.

Filestore adds `OpenScratch` and `EnsureScratchDirectory` for explicitly
disposable files and directories. Go's `os.Root` owns containment and traversal;
callers own returned file handles. These operations promise no durable commit.
Core and source packages admit caller-owned document/array extents and canonical
source identities beyond former product-size defaults. The defaults remain
bounded. Capabilities uses direct standard-symbol lookup. Associated module,
Git, line and source-record call sites follow the shared contracts.

The cgo setting is implemented inside Primitive using the small mechanism from
Go x/tools (`18332fec72972efbb8ab9881984fec2d8cfc2b58`), with its BSD notice retained.
It sets Go's private `go115UsesCgo` flag after checking its shape, then uses the
real Go checker. It never substitutes `FakeImportC`. This private setting is a
specific future-toolchain compatibility risk, covered by checker tables and a
real cgo compilation regression. Disabling the setting fails both tests.
There is no dependency on a private x/tools fork or local module replacement.

Integration checks caught the widened array counter in Attest's test oracle
and oversized JSON whitespace previously refused by Core on behalf of Attest
and Exchange. Those package-owned admission limits are retained at their
public decoders. Imported fixture ownership, diagnostic detail and analyzer
findings were corrected. Exact EOF comparisons have narrowly documented
waivers because joined EOF plus read failure must remain a refusal.

Process changes include faithful Go environment projection, native process
identity limits, Windows liveness/reaping fixes, full-width exit observations,
optional owned peak-memory observations, and bounded Plan transport closure.
Core also retains digest overflow admission, reader failure causes, strict
projection validation, raw path admission and endpoint extent fixes. Earlier
benchmark and fuzz evidence is preserved in the package review reports; those
reports describe their recorded snapshots, not fresh measurements of every
change in this integration. Contextstate production is unchanged; its expanded
hostile tables and benchmark harness are included, with the remaining sweep
work deferred.

The integration was built against swissKnife through a temporary module file,
without changing swissKnife's dependency declarations. Publishing this tag
makes the APIs available; it does not automatically upgrade consumer pins.

Verification results and performance evidence are recorded in
`release_v2026.1.21_evidence.json`. Local verification is not an independent
acceptance receipt. Raw command logs, source snapshots, profiles and binaries
remain ignored locally; reports and their digests accompany the release.

All 61 module packages have passing execution evidence across the native full
run and the final Attest/Compass/Version refresh. The full run initially failed
Attest's signature extent regression; that attempt is retained, followed by the
passing affected-package race run. Three credentialed GCS tests remain skipped.
Seven expected negative child-test events from Testserial are preserved in the
raw full-run counts; Testserial itself passed. GoToolchain, Core, Contextstate,
Filestore and Exchange passed their separate full race suites. The requested
module analyzers passed, including the unchanged four goconst admissions, and
the final affected packages received analyzer refreshes. Linux/amd64 and
Windows/amd64 compiler test binaries compiled; these are not native runtime
results. Native execution was Darwin/arm64 with Go 1.27.1.

Fresh benchmark commands requested 30 seconds each, with matching CPU profiles,
allocation profiles and test executables retained. These are single integration
samples, not a distribution establishing a timing trend:

| Workload | Iterations | ns/op | B/op | allocs/op |
| --- | ---: | ---: | ---: | ---: |
| Compiler production/internal-test variant check | 9,165 | 4,748,768 | 2,634,692 | 26,352 |
| Mixed standard-symbol resolution | 2,361,436 | 16,276 | 7,329 | 50 |

The compiler allocation profile attributes 98.82% cumulatively to Go's export
importer. The symbol profile attributes its allocations to the selected rule
slices. Those profiles provide specific targets for later work; this release
does not invent another compiler or retained compiler cache to remove that cost.

The handoff also retains three historical baseline symbol samples
(60,130 / 62,355 / 62,676 ns/op; 158,723 B/op; 501 allocs/op) and three historical
candidate samples (14,907 / 15,042 / 15,054 ns/op; 7,329 B/op; 50 allocs/op),
each requested for 30 seconds with CPU/memory profiles and matching binaries.
Their benchmark harness hash matches the integrated harness. These are earlier
source snapshots, identified separately in the evidence, rather than fresh
before/after measurements of this entire release. The fresh allocation result
matches the historical candidate; timings remain machine/run-specific.

The new scratch fuzzer initially exposed an oracle error at mode 0370: a native
permission refusal before chmod may leave the MkdirAll mode filtered by umask.
The corrected oracle demands exact permissions on success, and native refusal
plus no added permission bits on failure. Its crasher is retained as a corpus
entry and named mode seed, and the scratch race regression and affected lint
checks passed afterward. No production scratch behavior was weakened to make
that test pass.

Final targeted fuzz campaigns passed, each with a requested 30-second budget,
one worker and a one-second minimization bound: scratch creation (16,152
executions), signature JSON (410,579), and Method JSON (163,716). The evidence
audit validated 56 command records, 8,726 unique source snapshots and 176
artifacts with no missing-file or digest failures. This audit checks the
recorded local evidence; it does not issue independent acceptance.
