# Cloudflare SDK implementation evidence

The source under review is `0e6c118e1d0c5db042da9054a3eee8a61e766254`.
Mac and Furnace independently checked out those committed bytes. These are
implementation execution facts, not an acceptance receipt. Independent review
and acceptance remain with the user.

## Contract advanced

Cloudflare Images and Stream have separate server-issued upload grants and
client transfer capabilities. R2 has server presigning and client
GET/HEAD/PUT/DELETE capabilities. Cloudflare owns its provider constants, keys,
validation, authentication, and error identities. Official source URLs accompany
provider limits; SDK custody budgets are identified separately.

HTTP runs through Exchange, files through caller-owned Filestore scratch, and
time through Temporal. The package's source/AST ratchet rejects direct effect
imports and foreign provider SDK dependencies. Media payloads stream through
bounded windows. Caller policy owns permissions, retries, application
idempotency, readiness, and lifetime media accounting.

These capabilities cover direct Images uploads, basic Stream uploads, incoming
Images/Stream webhook authentication, and individual R2 objects. They do not
claim the complete Cloudflare administration API, Stream tus/resumable uploads,
R2 multipart, or queue consumption. Provider integration proof uses local TLS
servers through the real Exchange path. No live Cloudflare smoke was performed.

## Failure evidence retained

The history archive includes every retained attempt, including compile/setup
failures and unsuccessful verification. Later passes do not replace them.

- Normal EOF was incorrectly wrapped on the authenticated replay path.
- Images custom-ID restrictions and opaque Stream webhook secret admission
  needed correction against the documented provider contracts.
- Scratch replay needed to enforce the caller's total body budget.
- Named Stream signature fields were incorrectly sensitive to member order.
- Upload completion publication raced with the transport's body reader.
- R2 grant ingress admitted an issuer identity outside credential custody.
- Furnace fuzzing found upload-grant nominal round-trip drift for an unescaped
  space. The minimized input is byte-identical to the promoted repository seed;
  `regression-binding.json` records the binding. The old production revision
  fails the new regression and the fixed revision passes it.

Deliberate discarded mutations suppress envelope error rejection, alter the R2
signing region, introduce a direct HTTP import, and change a retained provider
error code. The corresponding tests fail. Their patches, outputs and execution
facts are retained. The envelope truth table exhausts the 12 flag/error/result
states with exclusive primary classes; representation hostility is also fuzzed.

The first URL regression test incorrectly assumed Go escapes a raw bracket.
Its failed result remains in history; the corrected expectation preserves Go's
documented URL ownership and the unchanged request meaning.

## Accounting qualifications

The final tests pass `-json` directly to `go test` and disable result-cache reuse
with `-count=1` or `-count=2`. An earlier harness incorrectly placed `-json` in
`GOFLAGS`, breaking child `go list -f` invocations. Those failures remain
recorded as harness failures, not product defects. An earlier full race run also
expired the existing 64 MiB Exchange test's 10-second budget after preserving
63,340,544 bytes. Its 120-second deadline now serves only as a deadlock backstop;
exact-byte and completion assertions remain unchanged.

`testserial` intentionally runs rejected nested tests through `testing.RunTests`.
Go emits their failure events into the parent stream even when the parent test
and package pass. Raw receipt counters retain these events. The final summary
identifies them separately and uses the actual parent/package/process outcomes;
it does not silently rewrite the raw counts.

Early gate launchers did not aggregate child exit codes. Their own zero exit
therefore never proves a passing gate. Child receipts are authoritative for
their individual commands. The final launcher aggregates failures correctly.
Unknown unavailable, timed-out and not-run counts remain explicit `null` values;
they are not invented zeros. Filtered runs prove only their stated filters.

Fuzz targets run serially, each with the repository's two-second active budget,
one-second minimization budget and two Go fuzz workers. Fuzz corpus caches may
be warm; test-result cache bypass does not claim a cold fuzz corpus. Only the
promoted reproducer is retained, not cache/minimization churn. No benchmark
improvement or live-provider acceptance is claimed.

## Retention and verification

`history.attempts.json` and `final.attempts.json` list all retained attempts in
execution order, including revision, dirty fact, command, scope, cache posture,
exit, signal and counts. Each archived receipt also records toolchain,
environment, changed-source hashes and stdout/stderr hashes and byte counts.
Early diagnostic dirty-tree runs have weaker source reconstruction than the
final clean committed runs; they are not substituted for final revision proof.

The two compressed archives retain raw outputs, receipts, mutations and red
source snapshots. Their manifests bind every archive member by SHA256 and byte
count, and bind the archives themselves. The bundler verifies receipt versus
manifest versus source bytes, then independently streams the written archives
to reject missing, duplicate, extra or altered members. `bundle.manifest.json`
seals the review files and tooling alongside those archive manifests.

The retained Python tools are operational evidence collectors, not production
SDK effects or independent acceptance authorities. Their absolute workspace
paths identify the environments used; the complete child argument vectors are
retained separately for reproduction.

The final gate status and exact results are in `FINAL_RESULTS.md`. No new
Witness waiver or compatibility path was added. Existing findings and live
provider skips remain visible and prevent an unconditional gate-green claim.
