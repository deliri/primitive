# Primitive v2026.1.106

Cloudflare R2 multipart uploads now have typed create, part, complete, and abort
capabilities. Each signature binds the bucket, key, upload identity, operation,
part number, and expiry. R2 protocol constants belong to Cloudflare; they do not
borrow another provider's SDK constants.

Part bodies pass directly through Exchange as `io.Reader`. They are never
buffered or staged in Filestore. Filestore remains the owner when the caller
actually needs local filesystem durability. Complete requests stream their
ordered manifest; the caller-owned manifest is bounded to R2's 10,000 parts.
Control responses have a 2 MiB bound, strict XML shape and coordinate checks.
Multipart layout planning derives one part extent at a time with scalar
arithmetic. It widens the requested extent when needed to respect the provider's
10,000-part ceiling and enforces the exact object and part limits in R2's
published limit footnotes.
ETags are provider receipts, not locally verified content digests. R2's optional
CRC64NVME response is validated and retained separately.

## Behavioral evidence

Mac evidence is retained on Furnace under
`/data/evidence/primitive-multipart-20261005/`. Receipts contain the full command,
revision, dirty-tree fact, source hashes, output hashes, attempts and scope.

- `multipart-streaming-restored-01`: `GOWORK=off go test -json -race -shuffle=on -count=2 ./cloudflare`,
  616 passing test results, zero failures or skips. Both runs executed.
- `multipart-streaming-mutation-red-01`: deliberately buffering the source before
  transport failed both streaming cases (three failed results including parent).
  That mutation was removed. A TLS peer must receive the prefix before the
  source releases its tail, so whole-body buffering cannot pass this test.
  Payloads are 32 KiB + 1 and 2 MiB + 3; maximum source read is 32 KiB.
- Four semantic fuzz targets executed for 15 seconds each, two workers: upload
  identity, signed coordinates, control responses and part receipts. Each
  passed. The initial signing attempt did not compile because a test used an
  incorrect Duration method; that failed attempt remains in the evidence.
- `cloudflare-live-04`: real R2 create, three streamed parts totalling 10,485,777
  bytes, completion, complete SHA-256 readback, another session's abort and
  refusal of a subsequent part, and object cleanup. One pass, no skips.
- `multipart-release-packages-03`: uncached Cloudflare and core package tests,
  2,305 passing results, zero failures or skips. Earlier attempts exposed missing
  ownership inventory entries, an existing image-deletion marshaler witness,
  and a test fixture outside ByteLength's admitted domain; all are corrected
  and the failed attempts remain retained.

Earlier malformed-response and live protocol failures remain alongside the
passing attempts. In particular, R2's live CRC64NVME completion field exposed a
decoder omission which is now covered by a regression fixture.

These results prove the SDK boundary. They do not prove browser CORS,
application authentication, session renewal, workout attachment persistence, or
application end-to-end uploads. Those belong to Kernel and the distro. No large
video transfer was required or performed. Repository-wide gates are not claimed
by this focused SDK checkpoint.

Provider references: [R2 uploads](https://developers.cloudflare.com/r2/objects/upload-objects/),
[S3 API compatibility and checksums](https://developers.cloudflare.com/r2/api/s3/api/),
[presigned URLs](https://developers.cloudflare.com/r2/api/s3/presigned-urls/).
