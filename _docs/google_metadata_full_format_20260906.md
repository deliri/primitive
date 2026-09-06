# GCE identity acquisition, v2026.1.17

The live Anvil VM's standard-format metadata token omitted `email` and
`email_verified`. Its full-format token included both. `GoogleCloudVerifier`
requires those signed facts, so requesting standard format made the actual
acquisition-to-verification path fail. Acquisition now requests full format;
verification and its audience, signature, issuer and account checks are unchanged.

The production mutation is one metadata query value. At production revision
43df86e8a03762065fc58c60b3b8f8572718036d, the local metadata HTTP provider
reproduces the observed standard/full distinction using real signed documents.
The regression fails before verification when acquisition returns the wrong
document. Foreign signatures, unverified accounts and foreign audiences retain
their refusal, certificate-fetch stage and zero identity after the fix.
Cancellation produces no token, metadata request or certificate request.

Existing signed hostile cases now cross real HTTP acquisition before verification;
malformed Authorization framing continues to exercise the verifier directly.
The signed semantic fuzzer also crosses metadata acquisition and checks exact
byte preservation, bounded typed refusal and independently pinned signed seeds.

Local development receipts are retained under
`/private/tmp/anvil-simplify-20260905/`, each with complete argv, source-tree
state, toolchain, exit status and stdout/stderr hashes and sizes:

- `primitive-gce-exact-handoff-red`: pre-fix failure at exact-byte handoff.
- `primitive-gce-exact-handoff-green`: corrected package race/shuffle run.
- `primitive-gce-release-tests`: googleidentity, version and compass, uncached
  with race detection and shuffle seed 732491.
- `primitive-gce-check-0`, `primitive-gce-check-1`: focused vet/staticcheck.
- `primitive-gce-doctrine-fixed`: focused doctrine lint.
- `primitive-gce-handoff-fuzz`: 30 seconds, two fuzz workers, semantic oracle.

These are local development results, not a live acceptance receipt. The next
proof is the published dependency on the real VM, its authenticated API callback,
and exact snapshot read-back through the kernel store.
