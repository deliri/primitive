# What Primitive already does

Read this **before** writing anything that touches the real world or crosses the
wire. If a capability is listed here, it exists: import it, extend it, or move
its owner. Do not write a second one.

The rule that puts something here: **if it crosses the wire it belongs in
Primitive; if it never leaves one side it is policy.** Both ends of every
transaction run this same stack, so a wire type has exactly one home.

| package | owns |
| --- | --- |
| `attest` | Ed25519 signing and verification of bounded typed canonical facts. Canonical-body hashing, domain separation, framing, detached envelopes, trusted-key sets. |
| `cloudidentity` | Acquiring one bounded outbound identity-token bearer from GCP metadata or regional AWS STS. |
| `contextstate` | Bounded observation of context terminal state. |
| `controlplane` | The signed documents a product control plane and its installed clients exchange: registration, installation certificates, check-ins, response headers, product status, verification results. |
| `controlwire` | The scalar values both ends must agree on byte for byte before anything else: request nonces, registration tokens and verifiers, revisions, policy cursors, route contracts, and the control exchange policy (timeouts, retry, redirect). |
| `core` | The typed bounded contracts shared by all of Primitive: error identities, byte counts, digests, streaming SHA-256 (`DigestWriter`, an ordinary `io.Writer` yielding one digest and the exact count of the bytes behind it), keys, offerings, build identity, paths, HTTP endpoints and status codes, strict JSON. |
| `currency` | Exact signed minor-unit values, checked same-currency arithmetic, ordering, bounded decimal and JSON projection. |
| `deploy` | Binding one authenticated release manifest to exact create-only GCS upload capabilities, returning confirmed provider facts. |
| `exchange` | Typed bounded operation policy over real `net/http` client and server boundaries: retry classification, exponential backoff, jitter, server hints, redirect confinement, idempotency and replay semantics, bounded JSON calls. |
| `filelock` | One advisory whole-file lock on one already-open file. Exclusive or shared, blocking or immediate; contention is a typed outcome rather than an error, and EINTR retries are handled here instead of in every caller. |
| `filestore` | Rooted, bounded, streaming durability over real `os.Root`/`os.File`: staged writes, commit, durable rename of an existing entry, read handles (`OpenRead`, for when a reader must be handed to something else), file and directory fsync, crash recovery, and path inspection (`Inspect` reports absent / directory / file / symlink / unreachable without following the final component, and carries the entry's modification time and byte count). |
| `fuzzfinder` | Finding Go fuzz corpus and crasher artifacts in a rooted directory with bounded memory and explicit partial accounting. |
| `garble` | Garble tool identity, deterministic seed derivation, and the Garble-owned prefix of a typed build intent. |
| `gate` | Turning one authentic lease assessment into permission to begin new paid work. |
| `hostfacts` | Bounded read-only facts about the current host: disk capacity and pressure assessment (a free-space floor at or above device capacity is refused as unsatisfiable), OOM banner classification. |
| `keygen` | Exact Ed25519 signing keys and bounded generic secret material from Go's production CSPRNG. **The entropy boundary.** |
| `lease` | Verifying and assessing one fixed-size OGS-signed commercial decision. Device identity, subjects, entitlements, grants, refusals, revocations. |
| `objectstore` | One exact bounded transfer through an already-issued S3, GCS, or Cloudflare Images HTTPS capability. Signed URLs, signed headers, upload targets, upload capabilities and their commitments. |
| `process` | Running one typed command over `os/exec`. |
| `receipt` | Authenticated accepted-evidence facts and one fixed-size monotonic watermark for later controlstate composition. Account identity. |
| `release` | Verifying a clean repository at an exact commit with exact build tools; deterministic fixed-target Garble build and process plans; artifacts, integrity, embedded build identity. |
| `shutdown` | Typed bounded cleanup and signal observation over context, time, and `os/signal`. |
| `temporal` | Exact typed nanosecond values and validated effects over real context and time. Instants, durations, observation. |
| `testserial` | Declaring why a Go test must remain non-parallel. |
| `timeproof` | Preparing and verifying bounded RFC 3161 timestamp evidence. FreeTSA and DigiCert authorities, policy OIDs, CMS parsing, nonces, refusals. |
| `upgrade` | Staging one authenticated release artifact into the unselected installation slot and exposing its exact command path. |

## Before you build

| about to build | look here first |
| --- | --- |
| signing, verifying, envelopes, trust sets | `attest` |
| random bytes, keys, secrets | `keygen` — never `crypto/rand` directly |
| an HTTP call with timeouts or retries | `exchange`, and `controlwire.ControlExchangePolicy` for control routes |
| a control-plane request or response | `controlplane` — registration and check-in are the two shapes |
| a nonce, token, revision, or route path | `controlwire` |
| writing a file that must survive power loss | `filestore` |
| stopping a second process from running | `filelock` — never a hand-rolled `syscall.Flock` |
| asking what occupies a configured path | `filestore.Inspect` — never `os.Stat` from a product |
| how old an entry is, for staleness or reaping | `filestore.Inspection.ModifiedAt` — the observation already holds it |
| how many bytes a file holds | `filestore.Inspection.SizeBytes` — regular files only, because nothing else has a meaningful one |
| handing a file's bytes to something that wants a reader | `filestore.OpenRead` — never `os.Open` from a product |
| moving an entry that already exists on disk | `filestore.Rename` — `Commit` only activates a stage |
| making a directory durable on its own | nothing: durability belongs to the activation that changed it, and a bare public directory sync is banned by the `filestore` architecture ratchet |
| uploading to S3/GCS/Cloudflare | `objectstore` — capabilities and commitments already exist |
| a timestamp, duration, or deadline | `temporal` |
| third-party proof of *when* | `timeproof` |
| deciding whether work is paid for | `gate` + `lease` |
| running a subprocess | `process` |
| hashing a stream you are already moving | `core.DigestWriter` — never a private `sha256.New()` accumulator |
| an error identity | `core/error_identity.go` |

## Keeping this honest

`TestCapabilitiesDocumentDescribesEveryPackage` reads the package directories on
disk and checks every one has a described row above. Adding a package without
describing it here fails the build, on purpose: a capability nobody can find is
a capability somebody rebuilds. There is no list of package names in the test,
so there is no second copy of the answer to drift.
