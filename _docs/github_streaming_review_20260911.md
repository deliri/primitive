# GitHub streaming boundary review — v2026.1.65

Primitive now lends each decoded tree path to the caller as a Go reader and returns typed metadata after entry EOF. It retains neither a complete path nor a repository collection. The 64 KiB rejection window explored earlier was removed after the user clarified that stream length must not become a policy ceiling. Its attempted implementation, failing and passing tests, and source snapshots remain in the evidence bundle as superseded work, not current proof.

## Shared agreement and migration

`TreeVisitor.VisitGitHubTreeEntry` now receives `*TreeEntryStream`. The visitor consumes decoded path bytes with ordinary `Read` or `io.CopyBuffer`, then calls `Observation()` after EOF. `TreeEntry` contains `PathSHA256`, `PathLength`, and the closed `Kind`; the former full-string `Path` field is deleted. Consumers explicitly construct their own path representation if needed. There is no compatibility reader or alternate legacy DTO.

The callback borrows the stream through return. Returning nil without consuming EOF is a typed refusal, and a retained unread reader refuses subsequent use. Early, zero-value, and failed observations remain zero with a typed error. Entry EOF means the whole entry object, including fields that follow its path, has been validated. An earlier path prefix can have reached the caller before a later refusal; it is untrusted until entry EOF. Successfully consumed entries may precede a failed or provider-truncated overall tree. Only the successful final `ReadTree` return establishes complete tree consumption. Primitive does not roll back caller effects or decide product completion.

The implementation uses fixed `bufio.Reader` scratch, one pending UTF-8 rune, a constant-size path-shape checker and digest, and the fixed provider object grammar. It does not parse arbitrary object graphs. Object-name and kind recognition can refuse a prefix that cannot match any published schema word; these are closed-domain checks, not configurable byte quotas. Ignored strings and whitespace are consumed incrementally. Paths are checked against the existing slash-canonical `core.SourcePath` contract without storing a segment. Core and the independent JSON decoder serve as differential fuzz oracles.

Current byte receipts still use `core.ByteLength` and its Go signed-int64 representation. This release does not claim that an individual 10-exabyte receipt fits that existing type. Supporting a wider aggregate receipt requires a separate counter-domain change; no size-policy ceiling or unbounded buffering is used to disguise that arithmetic limitation.

## Fixed defects and retained ratchets

- Conflicting continuation links are all checked. A valid first link cannot hide a conflicting later page. Identical replay is idempotent, and maximum-page responses with no next relation remain neutral.
- Tree visitor panics now cancel the download, close its pipe and join its worker before propagating. The original failure and corrected result are retained. Real local HTTP cancellation tests remain separate from the injected transport used to deterministically prove panic cleanup.
- A null completion flag cannot masquerade as false. Truncation errors retain the GitHub response identity and unexpected-EOF identity; EOF is reserved for a consumed delimiter.
- Unicode, escape, surrogate, numeric, duplicate-field, wrong-type and lifetime boundaries have named tests. Large paths, ignored strings and whitespace are generated incrementally, consumed completely, and checked with independent path-byte digests and exact counts.
- Deliberately wrong entry-kind, UTF-8-admission and credential-alias mutations failed their tests and were discarded. The mutation identity and all execution facts remain retained.
- Public head, page, tree, file and archive paths have semantic fuzz coverage. Reference, user-agent, identifier, credential, pagination-header, JSON-string and streamed-path ingress targets are included in the typed inventory. Core path parsing and independent standard-library JSON/RSA operations are oracles; a mere absence of panic is not the contract.
- The unchecked-error baseline was removed. Expected peer-close errors in deliberately interrupted fixtures are explicitly classified and logged. Unexpected write errors fail the fixture.
- A duplicate table row, quota-only accounting, and the obsolete small archive-ceiling test were retired. Tests of real large transfers remain. There is no product classifier or scoring lattice here; exact provider facts and closed entry kinds are projected without reconciliation, deduplication or retained product state.

The layer map is admission/schema, HTTP capture, incremental JSON/path decoding, synchronous visitor ownership, exact transfer observation, and provider continuation binding. Each changed boundary has local positive, refusal and neutral behavior. This package neither writes an artifact manifest nor owns a ledger, reporter or CLI. Its returned observations remain caller-owned facts. The external evidence harness separately retains and seals its execution files.

## Execution evidence and acceptance boundary

The append-only bundle is `/work/engineering-evidence/primitive/github-closure-20260910`. It retains revision and dirty-tree facts, complete commands, toolchain and machine identity, source bytes, stdout/stderr, every failed attempt and semantic mutation, and benchmark profiles/binaries. The release tag is derived from `compass/config.json`, not an independent version counter.

The final phase runs the discovered GitHub scope with `-count=1`, doctrine lint, vet, staticcheck, errcheck, full-module compilation, and race tests for GitHub plus Compass. All GitHub benchmarks run serially with a 30-second duration and allocation reporting after tools/tests; all discovered GitHub fuzz targets run serially after benchmarks with an explicit 10-second, one-worker budget. Generated-path benchmarks vary the declared path length while retaining fixed consumer scratch, so their allocation evidence tests memory growth rather than a claimed code-only speedup. No favorable-sample selection or baseline percentage claim is made.

The final external verification report, rather than this prose, identifies the exact committed execution and gate results. The manifest must match each retained reference and the on-disk bytes. Historical unavailable/not-run denominators remain explicitly unknown; missing counts are never passed tests. Local integrity verification is not independent execution acceptance. No live GitHub service, consumer upgrade/build, or whole-repository test success is claimed. Existing broader core/claim architecture baselines remain separate from this package gate.
