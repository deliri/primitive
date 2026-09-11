# Timeproof response custody completion — v2026.1.71

This closes the response custody, signed-attribute collection and certificate-count work left by v2026.1.70. It does not restart the historical package queue.

## Mechanical contract

Verify borrows the caller's DER response for the duration of the call. It retains SHA-256 and the exact byte extent, plus the nominal request, instead of copying or retaining the response. There is no response-size admission quota. AuthorityEvidence.ResponseBytes is replaced by ResponseDigest and ResponseSize; its JSON carries response_sha256 and response_bytes instead of response_base64.

A persisted metadata document is not authenticated evidence by itself. AuthoritativeTimestamp.Restore takes a RestoreRequest containing the document, caller-owned response bytes, and an independently expected message digest. It runs real verification, compares exact response identity and all persisted authoritative facts, requires canonical metadata, and changes the receiver only after success. The former UnmarshalJSON method is removed; callers explicitly retain their response and supply it when restoring.

Signed attributes use borrowed DER spans and fixed scanner scratch. Required-attribute lookup distinguishes absent, unique and duplicate without accumulating unknown attributes. The old 32-attribute ceiling is removed. Certificate framing is scanned one certificate at a time, with duplicate detection by rescanning prior DER spans; the old 16-certificate ceiling is removed. Rescanning trades time for parser storage.

Cryptographic verification remains with Go's crypto and crypto/x509 libraries, including authentication back to the existing pinned root. The user explicitly chose this exception to the constant-memory preference. X.509 certificate pools, decoded certificates and chain candidates retain library-managed working memory. This release does not claim end-to-end O(1) cryptographic verification, an io.Reader response API, or arbitrary-width byte receipts. Verify still accepts caller-owned []byte input.

## Evidence and scope

New local triads cover exact response identity, missing metadata/source, forged extent, independently mismatched subject, altered signature, attribute neutrality/cardinality, certificate-set neutrality and duplicate refusal. Authentic provider fixtures drive the real CMS/X.509 verification path. Allocation tests compare short and long attribute scans. The external-door inventory includes Restore, and a new semantic attribute fuzzer checks framing and required-value cardinality against standard-library representations.

Four deliberate semantic mutations are retained as failed executions: reinstate the 128 KiB response quota, reinstate 32 attributes, reinstate 16 certificates, and hash absent response bytes instead of the actual source. Each fails its named regression test; all mutations are discarded.

The requested repository-wide commands also exposed stale architecture inventories, a stale AWS response-quota assertion, two NilAway findings, and security-analysis issues. Those are fixed in the same completion. The deliberately uncompilable Go-analysis fixture is now created in its test-owned temporary directory, preserving the real compiler-refusal test without breaking repository scanning. The package-tool installer now installs the standard deadcode backend required by the pinned Witness wrapper.

Execution artifacts are retained outside source control at /work/engineering-evidence/primitive/final-completion-20260911. Attempts retain source hashes, revision/tree state, command, environment, output digests and exit facts. Initial missing-tool attempts, failed tests, and interrupted tool runs remain visible. Final runs bind the committed release bytes. JSON test events distinguish pass/fail/skip; unavailable or interrupted analysis is not a pass.

Fieldalignment and gocyclo findings reproduce on a detached clean v2026.1.70 checkout. These metric baselines remain explicit; this completion does not reorder hundreds of unrelated structs or reopen the historical package sweep. Plain deadcode reports no main packages in this library module; the -test run supplies executable test roots. Unreachable declarations are reported explicitly rather than treated as proof that public library APIs should be removed. Repeated strings reported by goconst do not justify coupling independent provider contracts.

These are retained execution and integrity facts, not an independent acceptance receipt. Benchmarks and fuzz targets run after tools and tests, serially, with exact configuration recorded. Final evidence reports determine the actual outcomes; documentation is not a substitute for those results.
