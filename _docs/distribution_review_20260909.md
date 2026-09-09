# Distribution review fixes — 2026-09-09

Status: ready for user review; uncommitted on Furnace.
Base: v2026.1.34, 0c20d585dcf1912af14e8eab44811542761ce92a.
This follow-up addresses /tmp/distribution_upgrade_20260909_findings.md.
The previous upgrade report and measurements remain historical evidence.

## Finding disposition

1. **Empty token closure restored.** SigningDomain.Validate again rejects an
   in-range enum whose token slot is empty. IsValid, MarshalText and MarshalJSON
   inherit that refusal. A deliberate missing-row mutation also disproved the
   review's statement that empty input would stay refused: ParseSigningDomain,
   ParseCanonicalText and UnmarshalJSON all accepted it before this repair.
   ParseSigningDomain now refuses empty input before searching the fixed table.
   Every one of the seven token slots was individually removed in a temporary
   production mutation; the completed closure tests pass for each missing slot.

   The request-domain owner now calls SigningDomain.Validate for the three request
   arms as well. Otherwise a RequestCommitment could still accept a missing request
   token even after restoring SigningDomain.Validate. That defect was reproduced
   separately and fixed before the final measurement.

2. **Direct Deploy source admission fixed.** UploadItemRequest.Validate uses
   Core.ReaderIsNil. The typed-nil reader that previously passed NewUploadItem now
   returns the Deploy contract identity. A real empty reader remains valid input;
   absent and typed-nil interfaces remain distinguishable test inputs.

3. **Receipt binding fixed; failed-attempt identity preserved intentionally.**
   Receipt.Validate requires Transfer.UploadCapability to be present and equal to
   the receipt's grant commitment. Both a confirmed foreign-capability transfer
   and a confirmed raw-provider transfer failed the new table before the fix.

   The suggestion to stamp Objectstore's capability only on success was not
   applied. Transfer explicitly describes an attempted operation, and its capability
   is the identity of that attempt, not a remote-confirmation claim. Transfer.Validate
   already requires CommitmentConfirmed, successful status and exact integrity.
   Removing the capability from failures would discard useful operation identity.

   New local tests prove a failed capability upload retains its identity while
   refusing Receipt.Validate, Transfer.Evidence and evidence JSON output.
   The existing Deploy prefix table now checks the actual UploadError at all eight
   failed positions: exact role/capability, CommitmentIndeterminate, no completion
   evidence, and only the prior confirmed receipt prefix. Objectstore production
   remains unchanged.

4. **Oversize diagnostic clarified.** Oversized signing-domain JSON now reports
   that it exceeds the JSON byte limit. Noncanonical admitted-size representations
   retain their canonical-form diagnostic. The stable JSON and Distribution error
   identities remain unchanged; no new error taxonomy was added for prose.

5. **Request-only validation happens before hashing.** CommitRequest now calls
   validateRequestDomain before creating the digest writer. The same owner enforces
   request membership and nonempty token validity in RequestCommitment.Validate.

No new production types, dependencies, state machines, registries, goroutines,
caches, or alternate execution paths were introduced. Production changes since
the reviewed source are confined to distribution/domain.go,
distribution/commitment.go and deploy/release.go.

## Tests and retained attempts

New tests: distribution/token_closure_test.go, deploy/receipt_binding_test.go,
and additions to deploy/capability_evidence_test.go. They use earned tables,
typed identities and real Objectstore/Exchange calls through a local injected
HTTP transport. Receipt tests deliberately assemble private receipts to attack
their validator; they do not claim those invalid receipts were emitted by Deploy.

The original red test draft had two build mistakes: a nonexistent CRC32C helper
and a comparison of a non-comparable UploadItem. Both failed builds are retained.
The compiled red run demonstrated the typed-nil source and both receipt identity
gaps. Missing-token mutations separately demonstrated domain/parser and request
commitment closure failures. Each mutation restored source bytes after its run.

An early benchmark attempt was interrupted to finish request-token closure.
Its stdout, stderr, exact source, result and explicit SIGINT reason are retained;
partial rows are historical observations, not final measurements or target-code
failures. The final phase was run after the revised tests/tools passed.

Final race evidence:

- deploy: 61 passing events, 0 failures, 0 skips; coverage: 87.9% of statements.
- distribution: 1491 passing events, 0 failures, 0 skips; coverage: 90.1% of statements.

Scoped go fix -diff, Vet, Staticcheck, Errcheck, Witness and production gocyclo <=10 pass. go fix suggested no changes. Distribution and Deploy test binaries compile for macOS arm64 and Windows amd64; they were not executed there.


## Benchmarks and profiles

All twelve benchmark workloads were rerun once on the final source, serially,
with -run=^$, -bench=., -benchmem, -benchtime=30s, -count=1, -timeout=20m,
and matching CPU/memory profiles plus the retained distribution.test binary.
The benchmark file is byte-identical to the prior reviewed candidate.

| Workload | Original ns/op | Prior reviewed ns/op | Final ns/op | B/op, prior → final | Allocs/op, prior → final | Final effective seconds |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| BenchmarkParseSigningDomain | 63.03 | 14.83 | 15.89 | 0 → 0 | 0 → 0 | 15.890 |
| BenchmarkCommitPublicationRequest | 1,615,162.00 | 1,640,097.00 | 1,634,079.00 | 202,115 → 202,066 | 3555 → 3555 | 35.938 |
| BenchmarkVerifyPublicationRequest | 5,698,247.00 | 5,810,280.00 | 5,773,884.00 | 679,765 → 679,739 | 12402 → 12402 | 35.602 |
| BenchmarkVerifyPublicationGrant | 4,524,219.00 | 4,544,515.00 | 4,504,286.00 | 578,105 → 578,084 | 9859 → 9859 | 35.444 |
| BenchmarkVerifyUpdateRequest | 87,768.00 | 87,668.00 | 87,522.00 | 2,210 → 2,210 | 50 → 50 | 35.987 |
| BenchmarkVerifyUpdateResponse | 14,227,690.00 | 14,665,406.00 | 14,094,964.00 | 1,814,013 → 1,813,697 | 32599 → 32598 | 38.268 |
| BenchmarkVerifyUpgradeRequest | 295,311.00 | 295,412.00 | 276,773.00 | 27,689 → 27,690 | 602 → 602 | 37.253 |
| BenchmarkVerifyUpgradeGrant | 436,763.00 | 446,576.00 | 432,271.00 | 49,121 → 49,110 | 940 → 940 | 37.560 |
| BenchmarkVerifyPublicationCompletion | 2,973,593.00 | 3,054,354.00 | 3,042,125.00 | 403,993 → 403,984 | 7434 → 7434 | 30.421 |
| BenchmarkUpdateResponseJSON/encode | 4,334,702.00 | 4,365,372.00 | 4,305,131.00 | 690,730 → 690,497 | 10875 → 10874 | 39.035 |
| BenchmarkUpdateResponseJSON/decode | 4,901,268.00 | 4,916,574.00 | 4,751,721.00 | 758,202 → 758,173 | 14626 → 14625 | 34.474 |
| BenchmarkSigningDomainJSONRefusal | 410,732.00 | 596.70 | 596.30 | 208 → 208 | 6 → 6 | 34.787 |

Final nominal benchmark budget: 360s; process wall: 412.272s.
Wall/30s flag: 13.74×; wall/phase budget: 1.15×.
The interrupted attempt emitted 3 completed rows before cancellation;
those rows remain in the machine evidence and are excluded from final comparisons.


Final profiles retain Go's existing work: Ed25519 field multiplication is
24.11s flat (5.30% of sampled CPU), SHA-256 is 6.85s flat (1.51%), and Go JSON
marshaling remains a major cumulative cost. ParseSigningDomain accounts for
10.01s flat (2.20%). Restoring token closure did not restore the former repeated
Validate/String calls inside the parser's search loop.

The final parse sample is 15.89 ns/op, 0 B/op and 0 allocations, versus
63.03 ns/op originally and 14.83 ns/op in the first reviewed candidate.
It reached Go's billion-iteration ceiling at approximately 15.89 effective
seconds despite the 30s configuration; it remains exploratory timing evidence,
not a thirty-effective-second acceptance sample. Oversized JSON refusal remains
596.3 ns/op, 208 B/op and six allocations (prior candidate: 596.7 ns/op,
208 B/op and six allocations; original: 410,732 ns/op, 262,533 B/op and eleven).

The final alloc_space profile totals 51.25GB sampled cumulative allocation,
including 14.78GB bytes.Clone and 6.95GB errors.Join. Release's artifactDigest
and manifestFactDigest remain 17.33GB and 18.91GB cumulative respectively.
Those aggregate amounts depend on how many operations each duration-configured
benchmark completed. They are not retained memory or a code-only allocation
regression. Per-operation counts in the table are the relevant comparison;
no validation or cryptographic work was removed to improve them.


One sample per revision is not a performance distribution or statistical
acceptance. Furnace remains shared with no isolated power/affinity posture;
host snapshots do not establish baseline/candidate governor parity. The original
completion workload lacked capability commitments, while both reviewed
candidates include them. That original-to-candidate row is not a code-only
latency comparison. alloc_space is cumulative sampled allocation, not peak RSS.
Effective per-row durations and iteration counts are retained; Go's billion-
iteration ceiling can end a fast 30s-configured parse benchmark early.

## Focused fuzz verification

The changed decoder and commitment paths were fuzzed serially after the final
benchmarks: SigningDomain's three public parsers, RequestCommitment JSON and the
independent canonical-frame oracle. Each target uses 30s, four workers, 1s
minimization and a 2m backstop; normal tests are filtered out. Persistent Go fuzz
caches remain enabled.

| Target | Executions | Process wall |
| --- | ---: | ---: |
| FuzzSigningDomainExternalDecoders | 681,051 | 33.476s |
| FuzzRequestCommitmentCanonicalFrame | 581,177 | 30.951s |
| FuzzRequestCommitmentExternalDecoder | 606,734 | 30.985s |

All three targets passed: 1,868,962 executions.
Nominal budget 90s; process wall 95.412s. Wall/per-target flag 3.18×; wall/phase budget 1.06×.


The earlier full sweep's seventeen-target, 5,988,342-execution campaign remains
bound to its original source. It is not relabeled as a seventeen-target run on
this revision. Ordinary Distribution and Deploy tests, including all fuzz seed
corpora, were rerun with race checking and -count=1 on the final source.
Unchanged document decoders did not receive another mutation campaign in this
focused review follow-up.

## Evidence and limits

Final Go/module-input digest: **425d7c32c8af5beecc323e49330a45364e3b05f33caf1102e6e76504d0d392cd**.
All final checks, profiles and focused fuzz runs match this digest. The tree is
dirty on the published ID base; this does not claim a new committed revision.

- [Machine evidence](distribution_review_20260909_evidence.json) lists all
  attempts, commands, source hashes, benchmark samples and raw artifact hashes.
- Raw follow-up root: /home/d/engineering-evidence/primitive/distribution-review-20260909.
- Prior full-sweep root: /home/d/engineering-evidence/primitive/distribution-upgrade-20260909.
- Original findings and the prior report/manifest are retained under the
  follow-up root, including their exact original bytes.
- Large binaries, profiles and content-addressed snapshots remain outside Git.

Full-module gates, consumer updates, native macOS/Windows execution and live GCS
uploads were not run. Deploy's complete package sweep remains queued. This is
author-produced verification for review, not an independent acceptance receipt.
Nothing has been bumped, committed or pushed in this follow-up.
