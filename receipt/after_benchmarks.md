# Receipt after — boundary audit, 2026-09-10

## Contract changes

Receipt's six structured JSON decoders now use Core's existing extensible byte
extent with their closed schema checks. EvidenceDocumentJSONMaximumBytes,
WatermarkJSONMaximumBytes, private whitespace allowances and the helper's byte
quota field are removed. Required fields, duplicate/unknown-field refusal,
canonical nominal identities, exact integrity facts and atomic receiver
replacement remain. Fixed identity widths and attainable canonical output
sizes are contract geometry, not product payload quotas.

Canonical output rejects typed-nil writers through Core.WriterIsNil. Every
acknowledged short prefix, native writer failure and invalid-payload absence
of output is checked. Go's writer remains the execution boundary; there is
no retry loop or custom buffer runtime.

Generation.UnmarshalJSON trims only Go JSON's four whitespace characters using
bytes.Trim, then retains strict positive, canonical decimal parsing through
strconv. Fractions, exponents, extra values, non-JSON whitespace, zero and
unrepresentable integers still fail without replacing the receiver.

Submission's DecisionDocumentJSONMaximumBytes depended on Receipt's deleted
quota. That quota and the corresponding decision encode/decode checks are
removed; its decision decoder uses Core's strict extensible structure decoder.
A real authenticated reuse-evidence fixture crosses a 1 MiB whitespace prefix
and interior gap, while malformed/trailing input preserves the existing
receiver. Upload/reuse union validation remains. Other Submission JSON doors,
nested Grant contracts and the rest of that package retain their existing
behavior; this is a necessary caller correction, not its full package sweep.

Receipt still signs through Attest and Go's Ed25519 implementation. It carries
fixed-size evidence facts and compares two caller-supplied fixed-size
watermarks. It does not read object contents, persist state or maintain a
workflow. No new dependencies, compatibility aliases or alternate parsers
were introduced.

## Memory and measured costs

Receipt JSON methods accept and return complete caller-owned byte slices.
Large JSON whitespace is admitted without a product-size quota, but whole
input ownership and Core/Go decoder allocation still grow with actual input.
This is not an O(1) whole-document JSON streaming API. The represented object
may be large without enlarging its Receipt: only its typed identity, native
extent, hashes and attestation are retained. Existing nested Attest/Core
contracts were not replaced or comprehensively re-audited in this slice.

| Case | Before ns/op | After ns/op | Before B/op | After B/op | Before allocs/op | After allocs/op |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| BenchmarkVerifyEvidence-8 | 104284.0 | 111580.0 | 3030 | 3030 | 62 | 62 |
| BenchmarkAdvanceWatermark-8 | 231.6 | 230.8 | 0 | 0 | 0 | 0 |
| BenchmarkReceiptBoundary/issue-8 | 205323.0 | 202267.0 | 3989 | 3989 | 74 | 74 |
| BenchmarkReceiptBoundary/encode_document-8 | 39275.0 | 39873.0 | 5409 | 5409 | 80 | 80 |
| BenchmarkReceiptBoundary/decode_document-8 | 91420.0 | 91259.0 | 11906 | 11908 | 251 | 251 |
| BenchmarkReceiptBoundary/write_canonical-8 | 18681.0 | 20976.0 | 2322 | 2322 | 48 | 48 |
| BenchmarkReceiptBoundary/decode_scope-8 | 6723.0 | 6803.0 | 849 | 849 | 21 | 21 |
| BenchmarkReceiptBoundary/decode_watermark-8 | 24451.0 | 25317.0 | 3181 | 3181 | 74 | 74 |

Allocation counts are unchanged across these eight workloads. Watermark
advancement remains allocation-free. Verification and canonical writer samples
are slower after the change; issuance is slightly faster. No across-the-board
speed improvement is claimed from one sample.

Before CPU samples include native Ed25519 field arithmetic, Go JSON processing
and Core strict validation. Before allocation samples are led by bytes.Clone
(~22.8%) and Go's jsontext.NewDecoder (~12.9%). The after CPU/memory top reports
are retained beside their profiles. This slice fixes admission and writer
safety without substituting a new implementation for those owned mechanisms.

## Hostile proof

The existing complete Receipt sources and tests were read. New tables exercise
large valid whitespace at six typed JSON doors, malformed/trailing inputs,
receiver preservation, nil writers, every short write prefix, and generation
token grammar. Authentication-order rows combine valid scope mismatches with
tampered or untrusted evidence and require verification refusal first.
Verification and watermark tables now check exclusive specialized identities.
Redundant watermark rows and the duplicate standalone execution triad were
removed; retained boundary cases check exact selected facts and zero failure
results. A reordered JSON fixture now actually reverses its members.

Source inventory and canonical wire-layout ratchets parse compiler-embedded
files instead of reading the filesystem directly. The JSON fuzz helper uses
a compiler-constrained pointer receiver and exact comparable values instead
of runtime interface assertions. Native hex decoding supplies an independent
identity admission oracle. Generated valid JSON probes prevent a rejecting
decoder from satisfying the arbitrary-input refusal checks. The signing-domain
fuzzer checks the exact published token. An independent numeric/flag oracle
checks watermark replay, advancement, rollback, scope precedence, and the exact
conflict reason, including zero and maximum representable generations.

Seven compiled mutations were killed and restored: Receipt's byte quota,
typed-nil writer panic, generation whitespace refusal, replay divergence
acceptance, scope comparison before authentication, JSON reject-everything,
and Submission's dependent decision quota.

Scoped Receipt and Submission race tests, go fix -diff (empty), vet,
staticcheck, errcheck and witness-lint pass. Receipt production and the changed
Submission decision file satisfy gocyclo <= 10. All Primitive production
packages build. Final Receipt tests compile for macOS arm64 and Windows amd64;
runtime execution is Linux-only. Full-module test gates were not run.

Five Receipt fuzz targets and Submission's existing JSON-door inventory target
ran serially after profiles for 30 seconds each, with four workers. The two
Receipt inventory fuzzers were rerun after reverting a test-dispatch
normalization that shifted canonical seed selectors to the next door.
Their final dispatch preserves the original typed seed's door. All runs remain
visible: eight 30-second target runs, six unique targets, 240 configured seconds.
fuzz-results.json retains actual elapsed times and reported executions; raw
output retains seed/cache progress. All final runs pass.

The initial hostile failures and the intermediate broken module build remain
recorded. Test-dispatch refinement occurred after benchmarks; production and
timed workloads are unchanged. Final scoped checks bind to that corrected
test source. Documentation and an inaccurate two-byte case label were updated
after execution; that case's input and oracle are unchanged.
Evidence on Furnace: /home/d/engineering-evidence/primitive/receipt-upgrade-20260910/.
Base: 28950f769830d5fadeee121cd8f8322a25796335 (v2026.1.42).
This slice is uncommitted for user review. Each run records complete argv, base
commit, dirty source hashes, retained content snapshots, toolchain, selected
environment, stdout/stderr, exit, elapsed time and source stability.
manifest.json hashes retained profiles, binaries, snapshots and raw records.
These are author verification records, not an independent acceptance receipt.

Go 1.27.1; Linux amd64; AMD EPYC 7282; GOWORK=off; GOMAXPROCS=8.
Machine and selected Go environment are retained. Shared host and uncontrolled
power posture; one sample per case per pass. Timings are observations, not a
statistical performance guarantee. Eight cases per pass ran serially with
30 seconds configured for each, -count 1 -benchmem and explicit CPU/memory
profile and retained test-binary paths. All cases exceeded 30 seconds of
estimated timed work; counts and estimates are in benchmark-comparison.json.
Profiles report cumulative CPU/allocation samples, not peak or retained heap.

R=/home/d/engineering-evidence/primitive/receipt-upgrade-20260910
Commands:
```sh
go test ./receipt -run '^$' -bench . -benchtime 30s -count 1 -benchmem -cpuprofile "$R/baseline-profiles/cpu.pprof" -memprofile "$R/baseline-profiles/mem.pprof" -o "$R/baseline-profiles/receipt.test"
go test ./receipt -run '^$' -bench . -benchtime 30s -count 1 -benchmem -cpuprofile "$R/after-profiles/cpu.pprof" -memprofile "$R/after-profiles/mem.pprof" -o "$R/after-profiles/receipt.test"
```

Fixtures and expected canonical bytes are built before timing. The existing
verification and watermark benchmarks now check errors and exact retained
results after their deterministic loops; they no longer only KeepAlive an
unchecked error. Six added workloads measure issuance, document encoding and
decoding, canonical payload output, scope decoding and watermark decoding.
The canonical sink counts acknowledged bytes without retaining them.
These checked benchmarks were installed before changing production and kept
unchanged between passes. There is no live provider traffic in these cases.

Command elapsed seconds: baseline-profiles=295.14, after-profiles=288.126.

The previous note remains historical below.

---

# receipt after

No production change this pass. Cost tracks admitted work, not a compiler ceiling.

## Measured

`go test -run=^$ -bench=. -benchmem -count=1 ./receipt`

```
BenchmarkVerifyEvidence-10      	   23139	     51238 ns/op	    3027 B/op	      62 allocs/op
BenchmarkAdvanceWatermark-10    	11067894	       115.0 ns/op	       0 B/op	       0 allocs/op
```

All benches use `b.Loop()` or `for range b.N` where the timer must pause, `b.ReportAllocs()`, and observe the result.
No `unsafe`, no C.
