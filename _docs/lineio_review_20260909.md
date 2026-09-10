# Lineio review follow-up — ready for user review

Review supplied: /tmp/lineio_upgrade_20260909_findings.md on Furnace.
Base revision: 161e900507f2b2f79f4b1c8feca923057304f060
Toolchain: go version go1.27.1 linux/amd64
Exact Go-input digest for this follow-up: 992675e659a296f4fb5fe110a8011d471717d4691bce4e6f49b632f89d9c59a6

## Finding dispositions

1. Producer validation suggestion: no redundant production scan was added.
   Go 1.27.1 ReadSlice explicitly guarantees delimiter framing; its full-window
   branch returns its nonempty buffer. checkedReader prevents caller
   ErrBufferFull from impersonating that branch and refuses invalid read counts.
   This is a documented standard-library contract, not an accidental behavior.
   The caller-side Fragment.Validate remains available for independently supplied
   typed fragments. Existing producer tests already validate emitted frames.
   New intentional mutations emitting an empty continuation or LF+More both fail
   those tests. This preserves Go ownership without rescanning every byte in
   production to re-prove the same framing.

2. Duplicate classification: fixed. ReadFragment adds ErrLineIOScan only when
   that identity is absent. Native identities and bytes-plus-error are preserved.

3. Completing fragment wording: fixed in Fragment's Go documentation and lineio.md.
   More=false completes the preceding More=true prefix. LF by itself can complete
   that same line; it does not imply a separate empty line. Consumers can process
   fragments incrementally without collecting a whole line.

A new nine-row hostile table reconstructs logical line completions across
CR-aligned windows, LF-only completions, real empty lines, exact-window EOF,
partial final windows and retained ordinary CR bytes. A deliberate mutation
dropping completing LF fails that table. Table collection is a finite test oracle,
not a production buffering strategy.

## Current verification

449 passing test events, including parents and fuzz seeds; zero skips or
failures in the final race run. Statement coverage remains 100%.
Scoped go fix -diff, go vet, staticcheck, errcheck, witness-lint and gocyclo
passed. macOS arm64 and Windows amd64 compiled; neither was executed.
No full-module gates ran. The installed witness-lint checked Primitive only.

All seven benchmark cases reran for 30 seconds each, serially, with CPU/memory
profiles and matching binaries. Both fuzz targets then ran serially for 30 seconds
with four workers and one-second minimization. Test/fuzz budgets and shared-host
limitations from the original report still apply. No terabyte transfer was run.

fuzz: elapsed: 30s, execs: 1641715 (0/sec), new interesting: 2 (total: 39)
fuzz: elapsed: 30s, execs: 1762565 (0/sec), new interesting: 0 (total: 7)

Current benchmark output:

```text
goos: linux
goarch: amd64
pkg: github.com/deliri/primitive/v2026/lineio
cpu: AMD EPYC 7282 16-Core Processor
BenchmarkScanStreaming64Lines-8     	11095188	      3354 ns/op	 114.48 MB/s	     248 B/op	       5 allocs/op
BenchmarkScanStreaming4096Lines-8   	  432424	     82054 ns/op	 299.51 MB/s	     248 B/op	       5 allocs/op
PASS
ok  	github.com/deliri/primitive/v2026/lineio	72.719s


goos: linux
goarch: amd64
pkg: github.com/deliri/primitive/v2026/lineio
cpu: AMD EPYC 7282 16-Core Processor
BenchmarkFragmentStreaming/Line64KiBBuffer64-8         	  957042	     38404 ns/op	1706.49 MB/s	     248 B/op	       5 allocs/op
BenchmarkFragmentStreaming/Line64KiBBuffer64KiB-8      	 1217778	     29694 ns/op	2207.08 MB/s	   65720 B/op	       5 allocs/op
BenchmarkFragmentStreaming/Line1MiBBuffer64KiB-8       	  339634	    104959 ns/op	9990.31 MB/s	   65720 B/op	       5 allocs/op
BenchmarkFragmentStreaming/CRLFSplit64KiB-8            	 1000000	     30979 ns/op	2115.56 MB/s	   65720 B/op	       5 allocs/op
PASS
ok  	github.com/deliri/primitive/v2026/lineio	139.580s


goos: linux
goarch: amd64
pkg: github.com/deliri/primitive/v2026/lineio
cpu: AMD EPYC 7282 16-Core Processor
BenchmarkReaderConstruction-8   	100000000	       338.5 ns/op	     200 B/op	       4 allocs/op
PASS
ok  	github.com/deliri/primitive/v2026/lineio	33.875s

```

Allocation remains fixed for fixed windows: 248 B/op and five allocations with
a 64-byte window; 65720 B/op and five allocations with a 64 KiB window, for both
64 KiB and 1 MiB input lines. Error classification and documentation changed;
these single shared-host samples are not a statistical speedup claim.

## Evidence

Raw evidence: /home/d/engineering-evidence/primitive/lineio-review-20260909
Manifest: lineio_review_evidence_20260909.json
Manifest SHA-256: 8f6e5cc2d5d30b8fe535048e93fe1b7ee85c4a30930da3ef7162081bba413496

Every attempt, mutation, raw result, exact command, source snapshot, profile and
binary remains recorded. The original sweep and its benchmarks remain historical
evidence in lineio_upgrade_20260909.md. This follow-up supersedes its current-source
claim. Subsequent Filestore changes must have their own dependency-validation
evidence; this digest is not silently reused for changed Go inputs.

No Witness repository work was performed for this follow-up. No version bump,
commit or push was performed. User review remains pending; the remaining Primitive
streaming audit continues with Filestore.
