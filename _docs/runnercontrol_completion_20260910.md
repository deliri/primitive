# Runnercontrol completion review — 2026-09-10

Release: v2026.1.54. Base: cb3b08c (v2026.1.53).
Worktree: `/work/code/primitive` on furnace, accessed through SSH.
Evidence: `/work/engineering-evidence/primitive/runnercontrol-completion-20260910`.

This review consolidates the earlier output-metadata, coverage-streaming and
build-event slices with the remaining runnercontrol ingestion and ownership work.
Primitive records mechanical execution facts. A passed process outcome is not a
product acceptance decision, and this review is not an independent acceptance
receipt.

## Boundary and memory contracts

* Go test JSON: the flat upstream event grammar now consumes fixed string
  fragments and a fixed native-float window. It does not accumulate a complete
  event or impose a diagnostic byte quota. Standard Go JSON validates and decodes
  each complete string fragment. Action and output metadata become closed enums.
  Package identities are SHA-256 commitments of the complete decoded UTF-8 name;
  escaped and literal spellings have the same identity. The compiler owns one
  digest entry per planned package and its explicit benchmark aggregate. Memory
  is constant in diagnostic length, not in planned package cardinality.
* Benchmark text: fixed token storage and checked decimal accumulation retain
  only typed measurements. Duplicate metric units are refused. Diagnostics do
  not become measurements merely because they have been consumed.
* Coverage: the fixed-storage coverage parser from v2026.1.52 remains in use.
  JSON and coverage compilers now reject writes and repeated seals after their
  terminal operation; a zero Go compiler refuses input instead of panicking.
* JUnit: one XML document is processed with the standard XML decoder and an
  owned pipe/parser lifecycle. Total-report and post-allocation token quotas are
  retired. Non-whitespace outside the root and concatenated roots are refused.
  **The standard decoder materializes its current XML token and nesting stack.**
  This is O(current token size + nesting), not a fixed-memory XML lexer; it never
  retains the complete report. Seal and Abort both join the parser. Failure and
  error markers retain their explicit precedence over skip/disabled markers.
* Control JSON documents: these APIs explicitly own complete nominal structs and
  canonical byte results. Their memory follows the owned value. Strict schema,
  duplicate-member, nominal validation and authentication checks remain; arbitrary
  top-level wire-byte quotas are removed. Fixed artifact chunk windows, nominal
  field bounds and caller-supplied resource limits remain mechanical contracts.
  `SourceAcquisitionRequestMaximumBytes` remains a documented caller route-budget
  default because Blink consumes it; it is not a decoder admission limit.
* Plans, authentication and sockets: process/profile/resource plans remain typed
  descriptions. Providers, repository implementations and durable artifact stores
  remain outside these decoders. Existing socket layer triads exercise HTTP and
  exact repository calls; this report does not call those injected repositories
  disk-persistence or manifest proof.

The deleted whole-line accumulator and production Go wire DTOs have no compatibility
replacement. Test-only upstream Go wire fixtures remain typed. The unused delivery
socket byte-budget argument and unused exported document quotas are removed.

## Accounting and ownership

A failed Go package followed by a successful process exit is a typed contradiction.
The classifier cannot manufacture a passed observation from failed, unavailable,
cancelled, expired or not-run accounting with no associated failure. An actual
nonzero process exit supplies mechanical failure even when the caller omits an
error value. Skips remain explicit; they are not counted as passes.

The public producer-to-classifier test exhausts the single-package state domain:
absent, active, passed, failed and skipped crossed with success, process failure,
cancellation and deadline. Its 20 rows assert the real parser's intermediate
accounting, the final outcome and the absence of extra measurements/artifacts.
The tests also remove the refusal as a one-fact mutation and require rejection of
contradictory counters. This finite state proof is separate from the existing
multi-package cardinality, wire grammar and benchmark-boundary tables; it is not
presented as a padded 50-row matrix.

Returned observations now own accounting attempts, coverage values and nested
scaling samples. Mutating the request cannot rewrite the observation, and mutating
the observation cannot rewrite its request. Existing copied benchmark/artifact
collections retain their nominal ownership.

## Ingress inventory and proof

`external_ingress_inventory_test.go` names all 70 public JSON decoders and their
semantic fuzz targets, rejects missing targets, and rejects stale inventory rows.
`ingress-inventory.json` retains the corresponding source-to-target table.
The three byte-stream doors are separately exercised by Go-event, coverage and
JUnit fuzz targets. Native-float and string-projection differential fuzz targets
exercise the private fixed-storage scanner through standard-library oracles.

The review adds direct scheduling-claim, machine-submission and source-acquisition
fuzz targets. Signed-document fuzzing uses actual verification, including foreign
signed seeds, rather than treating structural validation as authentication. The
receive-only source bearer is round-tripped by explicitly constructing the real
issue-side projection; the receiver does not acquire a disclosure marshaler.

Red executions retain the original long-JSON refusal, duplicate metric defect,
accounting alias/contradictions, zero-compiler panic, document extent refusals,
JUnit framing failures and failed-package/zero-exit mismatch. Deliberate discarded
mutations additionally expose coverage/scaling aliasing, a growing parser input
carrier, native-float overflow acceptance and an authentication bypass that admits
a foreign-signed cleanup document. Their commands, changed source bytes
and failures are retained alongside green runs. Compile/setup mistakes and lint
failures are retained, not replaced by later successful attempts.

The package's production-struct role inventory includes every new stream carrier.
Strict `errcheck -blank -asserts` findings recorded by the v2026.1.51 review have
been repaired, including nominal fixture construction and authentication setup.

## Benchmark evidence

One baseline and one candidate pass used Go 1.27.1 on furnace, the same checked
Go JSON workloads, `-cpu=1`, `-count=1`, and `-benchtime=30s`. CPU/memory profiles
and binaries are retained as `json-before.*` and `json-after.*`.

| Diagnostic | Revision | ns/op | B/op | allocs/op |
| --- | --- | ---: | ---: | ---: |
| 1 KiB | baseline | 15,743 | 8,856 | 64 |
| 1 KiB | candidate | 20,949 | 3,112 | 30 |
| 64 KiB | baseline | 229,994 | 404,010 | 70 |
| 64 KiB | candidate | 883,071 | 8,470 | 368 |

The candidate trades CPU time for avoiding whole-event storage. Cumulative
allocation is not the same fact as live working memory. These are single-pass
local observations, not a performance trend or acceptance comparison; an
independently controlled power posture was not captured.

## Execution accounting and scope

Every retained run records its complete command, working scope, toolchain,
environment, revision and dirty state, source hashes and content snapshots,
stdout/stderr hashes and byte counts, exit/signal, cache posture and available
test counts. `-count=1` bypasses result-cache reuse. Unavailable counts are marked
unavailable rather than synthesized as passing counts. Discovery retains the
complete fuzz target set; the sweep runs each target once with a configured
10-second budget and four workers, serially after the benchmark. Earlier targeted
30-second exploratory fuzz phases remain separate execution facts. All 30
discovered targets passed that sweep. Subsequent signed-container oracle edits
receive an explicit additional phase for their four affected targets.

Final commands cover package tests, race tests, witness-lint, go vet, staticcheck,
strict errcheck and module-wide build. The final committed-revision checks and
fuzz verdict live beside the append-only run receipts. The artifact manifest
includes reports, profiles, binaries and retained execution facts and excludes
scratch/cache churn. No module-wide test success is claimed: previously recorded
failures in untouched core, awsidentity and deploy remain outside this package
review. Consumer version/vendor migration and independent runner acceptance
remain separate work.
