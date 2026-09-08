# Hostfacts upgrade review

The later [bug-report follow-up](review_followup.md) contains the current changes,
verification and profiled measurements. The evidence below describes the earlier
upgrade checkpoint.

Hostfacts and Temporal are ready for user review. This report records author
work and local evidence; it is not independent acceptance. No version bump,
commit or push has been performed for this batch.

## Production changes

Six defect or validation-hardening classes were demonstrated against the prior
implementation. They are not six independently reachable public regressions:

1. On hybrid Linux cgroup setups, the reader preferred a unified v2 membership
   even when the memory controller was explicitly attached to v1. Explicit v1
   memory ownership now takes precedence, matching Go's runtime cgroup logic.
   Both line orders, duplicate memberships and malformed surrounding records
   are exercised through real bounded files and Filestore.
2. `DiskAssessment.Validate` accepted a free-space floor at or above total
   capacity although construction rejected it. The complete rule now belongs
   to `Validate`, and construction uses that rule once.
3. `Hostname.Validate` enforced only nonempty text. It now owns the existing
   253-byte, UTF-8 and control-byte rules previously confined to OS admission.
   This closes validation of corrupt private values; the public OS constructor
   already rejected these inputs.
4. The private bounded reader admitted a maximum that could overflow its spare
   byte allocation. It now rejects maxima beyond the existing 1 MiB contract
   before reading or allocating. Zero maximum still admits an empty source.
5. Cgroup limit folding now validates membership and mount facts, including
   agreement on the cgroup version, before reading files. Previously a bad
   internal combination could look like an absent limit. Normal public parsing
   already validated the component facts; this is a crossing-boundary ratchet.
6. The private attached-terminal constructor returned a nonzero invalid value
   with its zero-width error. It now returns zero on refusal. Native terminal
   observation already represents zero-width attachment without geometry.

The first hostile run recorded 16 failing leaf cases and five failed parent
events across the first five classes. The terminal run added one failing leaf
and one failed parent. All red attempts and exact source snapshots are retained. Intentional package and module Go fix
changes are recorded with their before/after source identities.

Profiles also supported removing code: the custom per-byte OOM banner matcher
was replaced by `bytes.Contains`, retaining only the fixed overlap needed across
32 KiB reads. Classification still consumes the exact declared extent after a
match. Completion, short reads, invalid reader counts, empty-read limits and
native errors remain explicit. An error accompanying the exact final byte count
has Go `io.ReadFull` completion semantics; premature errors remain refusals.

Two `bytes.Cut` operations replace allocating membership splits. Valid unescaped
mount paths avoid the large escape-decoding buffer. Canonical OOM JSON uses
`strconv.AppendQuote` into a fixed array. There is no new Hostfacts exported API,
provider abstraction, production global state or execution runtime. File reads
remain Filestore calls; native host observation remains in its OS adapter.
Streaming scans take O(n) time and fixed auxiliary memory. No claim of O(1) time
for reading arbitrary input is made.

## Hostile tests and fuzzing

The complete 2,214-line local testing protocol was read before test edits. Direct
tables cover validation/construction agreement, hostname boundary bytes,
oversized reader declarations, hybrid controller ownership, terminal attachment
and width, process-environment preservation and exact streaming read accounting.
Every boundary split of both OOM banners is checked alongside one-byte pattern
corruption. A detected banner cannot hide an unread or failing declared tail.
Real PTY and descriptor tests check the kernel result and retained ownership.

Percentage construction exhausts the uint8 carrier; projection fuzzing uses an
independent integer-ceiling oracle. Whole-file cgroup fuzzing independently
selects controller ownership and rejects duplicates within the existing bound.
The path adapter fuzzes exact delegation and error identity at the Core boundary.
Existing JSON fuzzers now use the closed canonical tokens as their oracle;
oversized classifier input reaches production rather than being skipped.

The struct inventory binds 30 actual production structs through typed fields and
compares their names with the production AST. Stale regular-file-tree entries and
the weak minimum-count test were removed. That retired effect belongs to
Filestore. Ingress inventories bind the percentage and whole-file membership
fuzz targets. Assertion-hiding terminal helpers and duplicated percentage/floor
examples were removed where the replacement table supplies the stronger proof.

All 12 deliberate source mutations were rejected by semantic test failures:
disk-floor admission, hostname bounds, maximum-size spare-byte protection,
hybrid controller ownership, duplicate membership, cgroup-version agreement,
OOM overlap, plain-banner matching, tail consumption, JSON canonical spelling,
terminal zero-on-refusal and struct inventory binding. No build failure is
counted as a killed semantic mutation.

## Full-module gate follow-ups

The broader gate found several issues outside Hostfacts that earlier scoped
package gates had not exposed:

- Temporal's timeout benchmark needed an explicit nil guard after its loop for
  NilAway. The timed workload is unchanged; a focused 30-second CPU/memory-profiled
  refresh preserves final-source evidence.
- Core's exact effect inventory still listed retired AWS clock and Filestore
  syscall imports/calls, and older Exchange listener and Temporal parser counts.
  Each changed entry was checked against the current production call sites.
- Seven existing HTTP/OS declarations were missing from Core's coherent-domain
  admissions. Their live compiler witnesses were added. The two named Windows
  error constants are now visible on every build; native Windows interpretation
  remains in the Windows adapter. Values and call sites are unchanged.
- Runworkspace's old cleanup test assumed permissions could be changed on an
  unreadable directory. Filestore's previously documented contract requires
  native read/write acquisition. The replacement table proves cleanup of
  readable trees and native refusal with unchanged restricted permissions.
  No permission-repair runtime or production cleanup bypass was added.
- `go mod tidy` classifies the existing x/net dependency as indirect; no version
  or dependency was added. Go formatting repairs spacing/import order in three
  older files, with a subsequent focused test refresh.

The first OS test attempts encountered a sandbox denial for `/bin/ps`. They also
exposed the stale permission expectation, which persisted outside the sandbox
and was therefore not dismissed as an environment failure. Two already-failed
initial campaigns were interrupted during Filestore cleanup; their logs and
SIGQUIT stack dumps remain recorded as interrupted failures. Later complete
runs use the necessary native access. The constants retry uses the repository's
canonical auxiliary-directory exclusions and its unchanged four reasoned
admissions; the first overbroad scan is retained.

## Verification and evidence

The accepted uncached module run passed all 61 packages. Hostfacts recorded
969 passing test events and Temporal 744, with no failures or skips in either.
Statement coverage is 89.9% for Hostfacts (originally 88.6%) and 95.2% for
Temporal. The shuffled race run exercised each package twice: Hostfacts recorded 1,938
passing events and Temporal 1,488. Counts include parent tests and fuzz seeds.
The full logs contain intentional failure events from Testserial's nested
`testing.RunTests` refusal probes; their enclosing tests and package passed.
These are kept in the raw evidence rather than reported as production failures.

All 12 Hostfacts fuzz targets completed their 30-second requests serially with
one worker: 3,967,899 executions, no failures and none left unrun. The final
artifact audit verified 91 execution records and 2,094 retained files without
mismatches. The named analyzer gates, build, module tidiness, formatting and
complexity checks pass; production complexity remains at or below ten.

Final counts, coverage, fuzz execution totals, gate outcomes, skip details and
source bindings are recorded in [upgrade_evidence.json](upgrade_evidence.json).
The [benchmark comparison](upgrade_benchmarks.md) reports every paired workload,
including unchanged controls and the superseded short baseline.

The gate scope is the requested Go fix, vet, Staticcheck, Deadcode, Witness lint,
errcheck, NilAway and constants checks across all 61 module packages, plus
production complexity, formatting, module tidiness, build, uncached module tests
and two shuffled race passes. This is not a claim that the entire canonical
release script, all-package benchmark/fuzz campaigns or external provider
acceptance ran. The three authenticated GCS tests require external fixtures and
remain explicitly skipped; local success is not live-provider acceptance.

Runtime testing is Darwin/arm64. Linux/amd64 and Windows/amd64 test binaries were
compiled; compilation is not native runtime evidence. Witness reports eight
existing waivers and seven waived findings; no waiver was added in this batch.
Raw logs, fuzz records, snapshots, CPU/memory profiles and matching binaries stay
ignored under `testdata/test-upgrade-20260907/`. Only source and reports belong in
the eventual reviewed commit.
