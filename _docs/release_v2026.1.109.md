# Primitive v2026.1.109 — streaming multipart receipts and Images delivery

Parent: v2026.1.108, `7fbdf37f8026ebfeedaa244ae536492ca3bac670`.

## Current contracts

`R2Client.CompleteMultipart` now takes `R2CompletedParts`, a synchronous typed
source. Each actual UploadPart receipt passes through encoding/xml, io.Pipe
and Exchange with direct backpressure. No receipt array, XML assembly buffer,
pre-count or upload registry remains in this SDK path. This is a clean API
change; consumers must pass the current source contract.

Control responses stream into the existing closed XML schema. The invented
1,024-byte ETag, 4,096-byte upload-ID and two-MiB control-document quotas are
removed. Validation still enforces the provider's actual ordinal, framing and
identity contracts. Working memory retains the current XML token and typed
fields; it does not grow with the number of receipts. A Temporal deadline,
pipe closure and join cover early refusal, cancellation and source failure.

`ImageDeliveryRequest` binds a caller-owned HTTPS origin, nominal public
account hash, image ID and predefined variant. `ImageDetails.PublicAddress`
requires matching observed public metadata. Projection does not prove DNS or
delivery. The account hash has no invented 128-byte cap. The predefined
variant retains Cloudflare's documented 99-character restriction.

## Development evidence

Root: `/private/tmp/primitive-multipart-evidence-20261005`. Every run retains
its complete command, base revision, dirty-source manifest, toolchain, exit,
event counts and hashed stdout/stderr. These runs preceded the release commit.

- `completion-stream-extent-red-01` and `image-account-extent-red-01` expose
  the invented scalar quotas. Their corresponding green attempts pass.
- `completion-stream-custody-01` hung on early refusal. Its own test process
  was stopped with SIGQUIT to retain stacks; it is failed, cancelled evidence.
  Attempt 02 then exposed a fixture that had not enabled HTTP full duplex.
  Attempt 03 proves real early refusal cancels and joins the source.
- `completion-stream-packages-01` caught a direct context timeout crossing
  the owned effect boundary. The fix uses Temporal. Attempt 02 and
  `completion-stream-race-01` each passed 2,626 test events across cloudflare,
  core, compass, version and authored Hammer claims, with no failures/skips.
- `completion-buffer-mutation-red-01` deliberately buffers output using a Go
  overlay. The TLS backpressure regression fails. Production was not patched
  with that mutation; the overlay and result remain retained.
- `completion-stream-live-01` passed one real R2 test: three parts totaling
  10,485,777 bytes, streamed completion, exact download hash, abort refusal
  and cleanup. It does not claim a larger live transfer.
- Six `stream-release-fuzz-*` initial attempts were blocked by sandbox socket
  or cache permissions. All six `stream-release-fuzz-02-*` retries passed
  with three-second target/minimization budgets. Those are distinct attempts.
- `image-account-extent-green-01` passed 25 focused events after removal of
  the account-token quota. The earlier package/race evidence predates that
  final change; release verification must name the committed revision.

Clean committed package, race, live and serial fuzz receipts are retained
separately. Fuzz budgets remain three seconds, with actual overhead recorded.
This release does not claim the complete Primitive repository gate, Kernel
end-to-end streaming, a responsive browser matrix or independent acceptance.
