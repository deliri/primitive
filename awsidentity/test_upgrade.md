# AWS identity production and test upgrade

Scope: `awsidentity` only, plus the UTC timestamp parsing support in `temporal`
explicitly requested during this review. This is not the temporal package sweep.
HTTP effects remain `awsidentity.Acquire` → `exchange` → Go `net/http`. Time
parsing and operation budgets cross `temporal`; its implementation uses Go.

The project-local `_docs/testing_protocol.md` governs this work. Repository gates
are deferred at the user's direction. The user authorized production fixes,
package commits, and continuing the package sweep.
Local executions are review evidence, not an independent acceptance receipt.

## Actual defects and applied fixes

1. A valid signed query followed by malformed percent escapes or a semicolon
   pair was accepted because `URL.Query()` silently dropped the offending pair.
   Four regression cases and the query fuzzer's seeds demonstrate the defect.
   The fix checks the error returned by Go's `url.ParseQuery`.
2. A valid provider XML root followed by another root, malformed XML, or arbitrary
   text was accepted because `xml.Unmarshal` stopped at the first root. Six
   initial regression cases demonstrate the defect. The fix retains
   Go's typed XML decoder and requires a single complete document, allowing XML
   whitespace, comments, and processing instructions around the root.

Both AWS fixes and the temporal integration are applied. The installed AWS
files exactly match the retained `proposed.overlay.json` effective sources;
`applied-source-equivalence.json` records that comparison. Historical profile
and fuzz runs keep their original review-build labels.

The **25 mutation checks are deliberately injected faults**, not 25 bugs found
in the original package. Twenty-two target AWS and three target the new temporal
code. All 25 were caught and discarded. The first 22 were followed by focused
checks of the strengthened raw-XML fuzz oracle, JSON capability disclosure, and
exact compact-time projection. Each mutation, selected test, source overlay,
exit status, and output is retained in `testdata/test-upgrade-20260906`.

## Owned boundaries and proof surfaces

| Boundary | Proof | Fuzzing | Measurement |
| --- | --- | --- | --- |
| Opaque UTF-8 audience | Exact bytes, byte ceiling, malformed UTF-8, zero refusal; no URL or product policy | `FuzzAWSAudienceExactUTF8` | Original audience workload and maximum audience |
| Signed request input and retained request | Every query enum arm, required/optional presence, emptiness, duplication; scope/date/region/signature/expiry binding; endpoint syntax; exact retained source; invalid owned fields | `FuzzAWSRequestAudienceBinding`, `FuzzAWSRequestQueryClosure` | `BenchmarkAWSNewRequest` |
| Client and fixed operation policy | Zero client; positive bounded timeout pair; one attempt; redirects rejected; invalid ingress produces no transport effect | Already validated internal capabilities; covered by hostile tables | Included in acquisition workload |
| Provider response | Typed namespace, cardinality, token, expiration and metadata mutations; exact token projection; reordered envelope; complete XML framing | `FuzzAWSAcquireProviderEnvelopeMutations`, `FuzzAWSProviderResponseSemanticClosure` | Minimum/maximum provider XML and acquisition workload |
| Bounded effect | Real local TLS request observation; redirect/status refusal; read/close failure; cancellation; actual response extent and limit probe; exactly one call and owned body close | Public `Acquire` fuzz targets use the Exchange client and a documented RoundTripper seam | Acquisition transport seam, explicitly excluding network latency |
| Opaque token and disclosure | Entire single-byte alphabet; padding, byte bounds, malformed representations; exact explicit bearer; all formatting verbs; typed JSON-v2 refusal | `FuzzAWSAcquireTokenProjection` and provider fuzz targets | Minimum/maximum bearer disclosure |
| Compiler-owned structure | Constrained data-flow inventory; all exported functions and methods discovered; typed fuzz bindings; synthetic new-door/struct/alias guards | Structural ratchets are tables, not external decoding | Not a timed workload |
| Temporal additions | Compact UTC calendar and signed-nanosecond extent; exact formatting or precision refusal; RFC3339 zero-offset admission | `FuzzCompactUTCExactTime`, `FuzzRFC3339UTCExactOffset` | New compact/UTC parser measurements and unchanged RFC3339 comparator |

Temporal parsing now constrains AWS dates to `temporal.Instant`'s signed
nanosecond range. This is a representational bound. No current-time freshness,
JWT claims, credential discovery, request signing, or product authority policy
is added. The opaque token need not be a JWT. Synthetic query fixtures prove
shape admission; they do not authenticate real SigV4 signatures or contact AWS.

## Execution accounting

The raw bundle preserves the initial tests (80.3% AWS statement coverage),
original benchmark reports and fixture, original production sources, every
subsequent test attempt, profiles, binaries, and deliberate mutations. Records
bind commands to the revision, dirty-tree fact, source hashes, toolchain,
workspace environment, output hashes, exit status, and test/subtest/seed events.
Parent and child test events are reported as events, never as independent cases.

Failed fixture attempts remain failures in the record. In particular, XML
marshaling tags overrode `XMLName.Space`; the fixture now uses Go's explicit XML
start elements so namespace mutations change the actual input. JSON-v2 correctly
refuses these structs with no exported fields; the test now requires its typed
refusal and no emitted bytes. These were test-authoring corrections, not extra
production defects.

A coverage attempt using a newly introduced overlay-only Go file failed during
instrumentation: Go's cover tool tried to open its absent working-tree path.
The failed execution is retained. It does not establish AWS coverage. Ordinary
review-overlay test execution succeeded. After installing the same files,
working-tree tests with coverage succeeded without an overlay.

## Final local review results

- Applied working tree: 966 AWS and 601 temporal passed test/subtest/seed events,
  zero failures, zero skips (`applied-final-tests.json`). AWS statement coverage
  is 96.8%; temporal is 90.8%. The earlier review-overlay run also passed.
- Before applying the two fixes: 945 passed and 21 failed events, zero skips
  (`review-final-working-tree-tests.json`). These are the cases and parent
  events exposing the two unapplied defects, not 21 separate production bugs.
  That failing run reached 96.4% statement coverage; coverage does not make a
  failing run successful. Initial AWS coverage was 80.3%.
- All eight sustained fuzz targets passed once with a 30-second requested
  budget and one worker: 2,128,356 Go-reported fuzz executions total. AWS runs
  used the proposed overlay; temporal runs used its working-tree implementation.
  Go's default fuzz cache was used; interesting-cache churn stays outside the
  evidence bundle. No new crasher required promotion.
- Twenty profiled benchmark executions retain 40 CPU/memory profiles and 20
  benchmark binaries. All 60 profile-analysis commands succeeded. Timings are
  observations under contention, not an established performance comparison.
- Repository-wide gates, race gate, and independent acceptance remain deferred.
  The package commit is authorized; these executions do not claim gate completion.

The applied AWS fixes and temporal integration have no remaining failures in
the package verification. Sustained fuzzing and profiling were not repeated
merely to remove the overlay: the installed production bytes are identical.
The broader repository package sweep remains in progress.

The audience table now includes 40 distinct byte/UTF-8 cases, including both
sides of encoding-width and byte-ceiling boundaries. Its final added table rows
were checked by the final package run; the already-passed fuzz callback did not
change. The request and provider surfaces have their own hostile matrices.
The complete byte and query-enum domains are exhaustively checked. UTC-only
parsing is a narrow adapter over Temporal's separately tested RFC3339 parser.
There is no evidence-verdict producer/classifier, fold, or durable ledger added
in this slice; the special 50-case evidence-classifier matrix is not the role of
this provider-validation package.

Source retention limitation: the earliest authoring runs retained source hashes
but not a full snapshot of every intermediate test edit. Original baselines,
all mutation overlays, profiled sources, and final source snapshots are retained.
The final working-tree failure re-demonstrates both actual defects against fully
retained sources. Intermediate fixture failures are preserved as execution facts,
not advertised as independently replayable acceptance proofs.

See `before_benchmarks.md` and `after_benchmarks.md` for the measured workloads,
allocation facts, profile attribution, timing limitations, and exact commands.
