# GCS transport shutdown repair

Local repair evidence; not an independent acceptance receipt or a clean full
repository gate. Base revision: acf8b675e3612ffcb4b535f076bf611db2d73438.
Unrelated dirty submission/submissionauth edits were preserved and excluded
from this checkpoint.

The response-boundary transport did not forward the standard optional
CloseIdleConnections method. GCS provider and authentication wrappers also
shared the process default pool without retaining its lifecycle ownership.
An SDK Close call therefore did not prove that these connections exited.

Exchange now delegates optional shutdown through http.Client, including the
standard no-op behavior for a base without a pool. GCS clones the configured
standard transport policy and retains its own concrete transport. Both the
authentication and provider paths use that pool. Construction failures close
it, GCSClient.Close closes it, and GCSCapabilityIssuer now exposes Close for
its owner to call. No retry, sleep, global-pool cleanup or goroutine-ignore
rule was added. Consumers must wire the issuer's new Close into their owned
shutdown path; publishing this method does not prove downstream wiring.

## Behavioral evidence

- Exchange local server: connection reuse is observed before close; nested
  response wrappers must prevent stale reuse afterward. Empty and repeated
  close retain standard behavior. Original code failed the reuse assertion.
- GCS local credential/provider server: a real lookup produces a typed absent
  result and zero metadata; Close must produce a server-observed connection
  closure. Original GCS source failed with zero of one connections closed.
  Nil, unconstructed and repeated close preserve typed contract refusal.
- IAM issuer: public construction, a real SDK signing-leaf refusal, and actual
  connection closure. This is explicitly not upload-capability issuance proof.
  Removing only the pool-close operation fails the closure assertion.
- Blink's registered photo-clear HTTP test with Firestore and real GCS passed
  including its unchanged goleak TestMain when using the candidate module.
  This is local candidate evidence, not a published dependency run.

## Retained attempts

Every identifier below names `/Users/d/code/.work-evidence/anvil-execution-ID/`
with stdout, stderr and receipt.json. Receipts contain argv, working directory,
base revision, dirty source digests, toolchain, exit, counts and artifact hashes.
These local capture records explicitly leave the planned unavailable/not-run
denominator unknown and acceptance false; they are not complete acceptance
accounting. Count=1 and count=2 test commands bypass Go result-cache reuse.

| ID | Scope/result |
| --- | --- |
| uauVoo | Exchange shutdown original: exit 1, 0 pass / 1 fail |
| fWopzV | Exchange forwarding repair: exit 0, 1 pass |
| 3I1xrz | GCS shutdown, constructor and inventory: exit 0, 3 pass |
| mYDwYx | Original GCS overlay: exit 1, 0 pass / 1 fail, closure backstop |
| 54VTie | go vet exchange + gcsobjects: exit 0 |
| RrUvG4 | witness-lint: exit 3, `run layer lint: invalid doctrine report`; unresolved tool failure, not clean lint |
| 19rviE | Full affected packages, race/shuffle/count2 before issuer test: exit 0, 20776 pass / 0 fail / 6 skip |
| xQxha8 | Three shutdown tests: exit 0, 3 pass |
| Giu184 | staticcheck exchange + gcsobjects: exit 0 |
| 4VPY1y | Issuer skip-pool-close mutation: exit 1, 0 pass / 1 fail |
| ug2AVE | Full affected packages after issuer test: exit 0, 10389 pass / 0 fail / 3 skip |
| 41pWbU | Final three shutdown tests, race/shuffle/count2: exit 0, 6 pass / 0 fail / 0 skip |

The three skipped functions are the live private download, authenticated GCS
lifecycle, and soft-delete-retention checks. They were each skipped twice in
the count2 run. No benchmark measurement or fuzz campaign is claimed here.
Existing fuzz seeds execute in the full package test; that is not fuzzing.

Mutation files are retained in
`/Users/d/code/.work-evidence/auth-username-http-3dta57/`:

- gcs-before-shutdown-client.go SHA256 a5564a61608593f9f2fd5cadedf818046aa4d3592763f15342d40a096651ca94
- gcs-before-shutdown-upload_capability.go SHA256 3797cb67b8fe87466e0fc1d4df08569dcb64bfbe46e6c0e1781ee734c7d13a09
- issuer-without-pool-shutdown.go SHA256 4308c758d63055feb1f4e70db296377b000dcbcdaa9724f4a697d0fe169ea227

The overlay maps sit beside those files. Original-source overlays do not
modify the checkout. The final source still needs independent verification;
the unresolved linter error and skipped provider checks remain visible.
