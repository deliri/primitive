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
| `controlplanetest` | Test-only issuance of genuine authority-signed installation certificate fixtures through the real Controlplane, Controlwire, Lease, Receipt, and Temporal contracts. Production cannot import it. |
| `controlwire` | The scalar values both ends must agree on byte for byte before anything else: caller request nonces, authority-issued authorization nonces, registration tokens and verifiers, revisions, policy cursors, route contracts, and the control exchange policy (timeouts, retry, redirect). |
| `core` | The typed bounded contracts shared by all of Primitive: error identities, byte counts, digests, streaming SHA-256 (`DigestWriter`, an ordinary `io.Writer` yielding one digest and the exact count of the bytes behind it), keys, offerings, build identity, paths, HTTP endpoints and status codes, the shared one-MiB strict-JSON ceiling, and the closed selection/position/continuation/page-limit contracts used by authenticated catalogs. |
| `currency` | Exact signed minor-unit values, checked same-currency arithmetic, ordering, bounded decimal and JSON projection. |
| `deploy` | Binding one authenticated release manifest to exact create-only GCS upload capabilities, returning confirmed provider facts. |
| `distribution` | Product-neutral signed agreements shared by release authorities and tools: exact-manifest publication requests, expiring fixed-slot upload grants, provider-evidence completions, installed-build update checks returning authenticated Latest, and exact-candidate upgrade grants projected directly into Deploy and Upgrade. It owns no routes, credentials, storage, product policy, or orchestration state. |
| `distributionauth` | Binding device-signed update and upgrade requests to an authority-authenticated installation certificate, then admitting only the nominated device key and exact installed build. |
| `exchange` | Typed bounded operation policy over real `net/http` client and server boundaries: retry classification, exponential backoff, jitter, server hints, redirect confinement, idempotency and replay semantics, bounded JSON calls. |
| `filelock` | One advisory whole-file lock on one already-open file. Exclusive or shared, blocking or immediate; contention is a typed outcome rather than an error, EINTR retries are handled here instead of in every caller, and every other native lock failure remains reachable beneath `core.ErrFileLockUnavailable`. |
| `filestore` | Rooted, bounded, streaming durability over real `os.Root`/`os.File`: staged writes, commit, durable rename of an existing entry, durable removal of one entry or one whole rooted tree (`Remove` and `RemoveTree`: absence is success, the parent is synced, a symlink at the named entry is removed and never followed), streaming rooted traversal (`Walk`: typed continue or skip per entry, native or bounded lexical order, symlinks never followed), read handles (`OpenRead` and `OpenStagedRead`), and linear external-producer staging (`ActivationRequest`, `OpenStageDestination`, `File`, `FinishStageDestination`, `AbandonStageDestination`) that proves exact size and inode before atomic activation. It also owns confined roots, fsync, crash recovery, typed inspection, custody stamps, durable confirmation, lock carriers, and held-handle standing. |
| `fuzzfinder` | Finding Go fuzz corpus and crasher artifacts through Filestore's context-aware rooted streaming walk, with bounded canonical retention and explicit partial accounting. It owns classification, never a second filesystem door. |
| `garble` | Garble tool identity, deterministic seed derivation, and the Garble-owned prefix of a typed build intent. |
| `gate` | Turning one authentic lease assessment into permission to begin new paid work. |
| `gcsobjects` | Authenticated Google Cloud Storage through the official SDK: typed create-only bucket provisioning with flat or hierarchical namespace, compiler-owned logical prefix/object composition, create-only served-media and stored-file upload, SHA-256-bound bounded read, canonical object addresses, and generation-matched permanent exact-object or prefix deletion. Flat prefixes name directories without fake placeholder objects; access remains bucket policy. |
| `hostfacts` | Bounded read-only facts about the current host: disk capacity and pressure assessment (a free-space floor at or above device capacity is refused as unsatisfiable), the rotational class of the block device backing a directory (`ObserveDiskRotation`: rotational, non-rotational, unavailable where no single block device backs it, unsupported off Linux), Go runtime memory (`AssessGoMemory`), physical memory (`ObservePhysicalMemory`), the effective workload memory limit under cgroups (`ObserveEffectiveWorkloadMemoryLimit`), directory tree measurement (`MeasureTree`), the running platform (`CurrentPlatform`), OOM banner classification, terminal attachment and column geometry of an open descriptor (a terminal that reports zero width is a terminal without geometry, never a fabricated detachment), and the platform's per-user home, configuration, cache, and temporary bases as admitted absolute paths (`UserHomeDirectory`, `UserConfigDirectory`, `UserCacheDirectory`, `TemporaryDirectory`). |
| `id` | Canonical time-ordered identifiers: UUIDv7 and ULID minted from one `temporal.Observation` and caller-drawn `keygen` entropy, canonical-text-only parse, strict JSON round trip. Byte order is time order in both. |
| `keygen` | Exact Ed25519 signing keys with a complete seed-custody round trip (`Seed` out, `AdoptSigningKey` back, `SeedSize` named), bounded generic secret material, one uniform `RandomUint64`, and bounded public random tokens (`RandomToken`) from Go's production CSPRNG. **The entropy boundary.** |
| `lease` | Verifying and assessing one fixed-size OGS-signed commercial decision. Device identity, subjects, entitlements, grants, refusals, revocations. |
| `objectstore` | Exact bounded S3, GCS, or Cloudflare Images whole-object transfers through already-issued HTTPS capabilities. `UploadCapabilityRequest` and `DownloadCapabilityRequest` bind receive-only bearers to exact streams before `Upload` or `Download` selects the provider operation. Transfers are O(1), enforce exact extent plus SHA-256 and CRC32C, optionally emit typed monotonic `TransferProgress`, carry provider version facts, and project URL/path/content-free `TransferEvidence` for a higher signed protocol. Issue-only bearer projections and typed non-secret commitments keep authority and client blind to one another. |
| `chit` | Authority-signed UUIDv7 custody tickets over versioned collections and O(1)-streamed manifests of accepted object receipts; retention-aware custody states; bounded authenticated newest-first catalog snapshots; and typed all/specific, start/after, page-limited queries matching `chit -all` or one chit ID. |
| `chitauth` | Binding a device-signed Chit catalog query to an authority-authenticated installation certificate, then requiring the query account to equal the certificate account before admitting the nominated device key. |
| `payment` | Authority-signed UUIDv7 payment receipts binding Primitive `currency.Amount` values, settlement times, service periods, and account/offering scope; bounded authenticated newest-first receipt catalogs; and typed all/specific, start/after, page-limited receipt queries. |
| `paymentauth` | Binding a device-signed Payment catalog query to an authority-authenticated installation certificate, then requiring the query account to equal the certificate account before admitting the nominated device key. |
| `process` | Running one typed command over `os/exec` after compiler-owned argument/environment count, individual extent, and aggregate projection bounds; duplicate exact-environment names are rejected in bounded linear work. Also resolves a bare command name to the absolute path `Request.Command` requires (`Resolve`). A name found through a relative PATH entry is refused, not corrected. A configured or derived absolute path gets the same runnability answer through `ResolveExecutable`, which refuses what the host would not run for this process. Owns the child's containment (process-group isolation and a closed cancel-signal choice), the running-child handle `Begin` returns for signaling and termination while the wait is in flight, the group-survivor sweep that outlives the reaped leader (`Execution.Sweep`), the calling process's working directory (`WorkingDirectory`) and own identity (`Self`), whether one process identity is still alive (`Alive`), the complete inherited environment (`AmbientEnvironment`), and O(1) lookup of one exact ambient variable (`LookupAmbientEnvironment`). |
| `receipt` | Authenticated accepted-evidence facts and one fixed-size monotonic watermark for later controlstate composition. Account identity. |
| `release` | Verifying a clean repository at an exact commit with exact build tools, with repository metadata and executable inspection delegated through rooted Filestore capabilities; deterministic fixed-target Garble build and process plans; artifacts, integrity, embedded build identity; pure installed-versus-authenticated-Latest evaluation returning current, available, refresh-required, or reassess-at as a closed typed selection; and the single compiler-owned `PrimitiveVersion` used as the current Git/release authority. |
| `retrieval` | Device-signed exact chit/object download requests and authority-signed short-lived grants binding one manifest entry to a receive-only Objectstore bearer commitment. A verified grant projects a typed `DownloadCall`, or `DownloadFile` streams directly into Filestore's exact temporary, verifies provider and stage evidence, and atomically activates the target while returning a recovery request only when caller resolution remains. |
| `retrievalauth` | Binding a signed Retrieval request to an authority-authenticated installation certificate, then admitting only the device key that certificate names. |
| `shutdown` | Typed bounded cleanup and signal observation over context, time, and `os/signal`; registration closes through one atomic one-shot and Run consumes one immutable fixed-capacity snapshot without a lifecycle state machine. |
| `submission` | Device-signed evidence declarations, portable `ManifestIntent` facts, authority upload-or-reuse decisions, and device-signed URL-free provider completion evidence. Grants and completions bind the exact request, capability commitment, authority nonce, integrity, provider result, expiry, and retention promise without disclosing source or bearer material. |
| `submissionauth` | Binding signed Submission requests and provider completions to the exact authority-authenticated installation certificate and original request, then admitting only the nominated device key. It owns no evidence or commercial decision. |
| `temporal` | Exact typed nanosecond values and validated effects over real context and time. Instants, durations, observation. |
| `testserial` | Declaring why a Go test must remain non-parallel. |
| `timeproof` | Preparing and verifying bounded RFC 3161 timestamp evidence. FreeTSA and DigiCert authorities, policy OIDs, CMS parsing, nonces, refusals. |
| `upgrade` | Crash-safe dual-slot upgrade: stream one authenticated newer release into the unselected slot, verify exact bytes/build/platform, expose only that candidate's command path for a product-owned diagnostic, accept only a typed passing report bound to the candidate, atomically promote, re-prove the primary, and remove the former slot only after success. The installed primary remains selected through every failed download, verification, or trial. |
| `wiring` | Deriving one bounded immutable runtime component graph from the actual objects a command constructed. Product-owned closed identities and direct dependencies must form one complete rooted acyclic graph; each component declares its compiler-owned Primitive package doors, and duplicates, missing roots/dependencies, disconnected components, test-support doors, and invalid identities are refused before a command reports ready. |

## Before you build

| about to build | look here first |
| --- | --- |
| signing, verifying, envelopes, trust sets | `attest` |
| random bytes, keys, secrets | `keygen` — never `crypto/rand` directly |
| persisting a signing key and reading it back | `keygen.SigningKey.Seed` + `keygen.AdoptSigningKey` — store the seed, the minimal secret; never slice a 64-byte private key with `crypto/ed25519` size arithmetic |
| admitting a 64-byte private key another party already speaks | `keygen.AdoptPrivateKey` — the trailing half is re-derived from the seed, never trusted |
| a local random identifier or device label that is not a control-wire nonce | `keygen.RandomToken` — a bounded public draw, all-zero allowed; control-wire nonces have the opposite zero rule and live in `controlwire` |
| a full-width random seed or salt integer | `keygen.RandomUint64` — never `rand.Int` from a product; a range-bounded uniform draw has no door yet, so bring that need to keygen rather than writing a modulo |
| an HTTP call with timeouts or retries | `exchange`, and `controlwire.ControlExchangePolicy` for control routes |
| the client `exchange.NewClient` demands, when the standard transport is all you need | `exchange.NewStandardClient` — never an `&http.Client{}` literal from a product |
| a registration or usage check-in | `controlplane` |
| permission to submit one declared evidence object | `submissionauth` authenticates the credentialed device request; `submission` binds the authority grant before `objectstore` transfers it |
| reporting one completed evidence upload | `submission` signs URL-free provider evidence against the exact request and grant; `submissionauth` binds that completion to the same authenticated installation and original request |
| listing or selecting one custody chit | `chit` owns the signed all/specific query; `chitauth` binds its account, build, and device signature to the installation certificate |
| listing or selecting one payment receipt | `payment` owns the signed all/specific query; `paymentauth` binds its account, build, and device signature to the installation certificate |
| provisioning one GCS bucket | `gcsobjects.CreateBucket` with `GCSBucketCreateRequest`; namespace is a closed flat/hierarchical enum and existing buckets are typed conflicts |
| composing an object directory or name | `gcsobjects.ComposeGCSRootPrefix`, `ComposeGCSChildPrefix`, and `ComposeGCSObjectName`; never concatenate slashes in a consumer or create fake zero-byte directory objects |
| a nonce, token, revision, or route path | `controlwire` |
| writing a file that must survive power loss | `filestore` |
| stopping a second process from running | `filelock` — never a hand-rolled `syscall.Flock` |
| turning a command name into something you can run | `process.Resolve` — never `exec.LookPath` from a product; the `filepath.Abs` every consumer wrote after it was unreachable, because Go returns `exec.ErrDot` instead of a relative answer |
| confirming a configured or derived executable path is runnable | `process.ResolveExecutable` — never `exec.LookPath` from a product, and never a stat, which glances at mode bits while the host answers with the process's effective identity |
| asking what occupies a configured path | `filestore.Inspect` — never `os.Stat` from a product |
| how old an entry is, for staleness or reaping | `filestore.Inspection.ModifiedAt` — the observation already holds it |
| how many bytes a file holds | `filestore.Inspection.SizeBytes` — regular files only, because nothing else has a meaningful one |
| whether a file's bytes are really allocated, for reserve and sparse checks | `filestore.Inspection.Allocation` — never `info.Sys().(*syscall.Stat_t)` from a product; unreported is a vacuously satisfied answer |
| what a file's mode is, for a durable record | `filestore.Inspection.Permissions` — a typed permission field, never an `fs.FileMode` whose high bits say "directory" |
| who owns a file | `filestore.Inspection.Ownership` — never `info.Sys().(*syscall.Stat_t)` from a product; that assertion lives in one platform leaf |
| turning an absolute path into a rooted request | `filestore.OpenParent` — opens the parent, names the entry, hands back a `Location` |
| turning operator-supplied path text into an absolute path | `AbsolutePath.ResolveText` against `process.WorkingDirectory` — never `filepath.Abs` then re-parse, which hides the kernel's working-directory ask inside a string helper |
| where a name really leads, links resolved, for an integrity comparison | `filestore.Canonicalize` — never `filepath.EvalSymlinks` from a product |
| handing a file's bytes to something that wants a reader | `filestore.OpenRead` — never `os.Open` from a product |
| handing a completed Filestore stage to an external verifier before activation | `filestore.OpenStagedRead` — it proves the stage still names the exact completed file |
| letting a standard-library-compatible producer stream into an atomic Filestore activation | validate one `filestore.ActivationRequest`, then use `OpenStageDestination`, `File`, `FinishStageDestination`, and its `CommitRequest`; call `AbandonStageDestination` on refusal |
| moving an entry that already exists on disk | `filestore.Rename` — `Commit` only activates a stage |
| removing one named entry | `filestore.Remove`; never `os.Remove` from a product. It never recurses, an already-absent entry is success, and the parent is synced so the removal is durable |
| removing a whole tree | `filestore.RemoveTree`; never `os.RemoveAll` from a product. A symlink at the named entry is removed, never followed, the OS owns traversal and buffering, and the removal is made durable |
| walking a tree under a root you hold | `filestore.Walk`; never `filepath.WalkDir` from a product. Typed continue or skip per entry, native or bounded lexical order, symlinks never followed, one fixed entry batch per open directory |
| making a directory durable on its own | nothing: durability belongs to the activation that changed it, and a bare public directory sync is banned by the `filestore` architecture ratchet |
| proving a name written before this process started is durable | `filestore.ConfirmDurable` — the one durability question no activation of yours can answer, most often asked after a restart |
| recording that a stored object was wanted again | `filestore.Touch` — never `os.Chtimes` from a product; it stamps and makes the stamp durable in one operation |
| the `*os.File` that `filelock.Request` demands | `filestore.OpenLockFile` — never `os.OpenFile` from a product; `OpenRead` refuses an absent file and `OpenAppend` hands back a handle a holder cannot rewrite its diagnostics through |
| uploading through S3/GCS/Cloudflare signed capabilities | `objectstore` — capabilities and commitments already exist |
| executing an authority-issued upload bearer | `objectstore.Upload` with one `UploadCapabilityRequest`; the request binds bearer, stream, media type, exact integrity, policy, and optional observer before transfer |
| executing an authority-issued download bearer | `objectstore.Download` with one `DownloadCapabilityRequest`; Cloudflare download is compiler-refused because that provider publishes upload only |
| showing exact CLI upload or download progress | `objectstore.ProgressObserver` receives monotonic typed `TransferProgress` with direction, completed bytes, and the already-declared total; returning an error stops the transfer |
| carrying provider-confirmed transfer proof without bearer, URL, path, or content | `objectstore.Transfer.Evidence` produces the issue-only projection; decode it as receive-only `TransferEvidence` before a higher signed receipt or chit protocol |
| turning an authenticated submission decision into the exact upload operation | `submission.VerifyDecision`, then `VerifiedDecision.UploadCall`; reuse decisions expose authenticated existing evidence and perform no upload |
| naming one object inside a large uploaded version | `submission.ManifestIntent` with one `UploadID`, `chit.CollectionID`, `chit.EntryName`, exact sequence, and total object count |
| listing uploaded custody versions or selecting one chit | `chit.NewQuery` with `All` or `Specific`, `Start` or `After`, and Core's bounded page limit; authenticate returned pages with `chit.VerifyCatalog` |
| listing payment receipts or selecting one payment | `payment.NewQuery` with `All` or `Specific`, `Start` or `After`, and Core's bounded page limit; authenticate returned pages with `payment.VerifyCatalog` |
| downloading one authenticated chit object directly to a file | verify the Retrieval grant, then use `retrieval.VerifiedGrant.DownloadFile`; it streams through Objectstore into Filestore and activates only after exact evidence matches |
| authenticated GCS bucket/object create, read, or permanent exact/prefix delete | `gcsobjects` — `CreateBucket`, `UploadMedia`, and `UploadFile` own the official SDK; products never import it |
| naming the public URL of a stored object | `gcsobjects` — `Address` on an accepted result, `ObjectAddress` from a bucket and name; never rebuild the provider URL from a copied host |
| a timestamp, duration, or deadline | `temporal` |
| a time-ordered unique identifier | `id` — `NewUUIDv7` or `NewULID` from `keygen.GenerateSecret` entropy and `temporal.Observe`; never `google/uuid` or a hand-rolled ULID from a product |
| third-party proof of *when* | `timeproof` |
| how wide the terminal is, for rendering | `hostfacts.ObserveTerminalGeometry` — never an ioctl or `golang.org/x/sys` from a product |
| the host's own name, for a device record | `hostfacts.ObserveHostname` — never `os.Hostname` from a product |
| the per-user configuration base | `hostfacts.UserConfigDirectory` — never `os.UserConfigDir` from a product |
| the current user's home directory | `hostfacts.UserHomeDirectory` — never `os.UserHomeDir` from a product |
| the per-user cache base | `hostfacts.UserCacheDirectory` — never `os.UserCacheDir` from a product |
| whether the disk under a directory spins, for I/O planning | `hostfacts.ObserveDiskRotation` — never `/proc` mount tables or `/sys` reads from a product |
| the expected 200 success status for an exchange call | `core.HTTPStatusOK` — never `net/http` status constants from a product; a transport response admits its code through `HTTPStatusCode.AdmitInt` |
| the scratch base for uniquely named temporary entries | `hostfacts.TemporaryDirectory` plus a keygen token for the name — never `os.MkdirTemp` from a product |
| deciding whether work is paid for | `gate` + `lease` |
| running a subprocess | `process` |
| isolating a tool tree so cancellation reaches all of it (POSIX hosts) | `process.Containment` with `IsolationGroup` — never a hand-rolled `SysProcAttr{Setpgid}`; Windows has no group signal and refuses the request rather than delivering it to one process |
| supervising a running child (signal, force-kill, hold it) | `process.Begin` and the `Execution` it returns — never `cmd.Process` from a product |
| whether a pid is still running | `process.Alive` — never `syscall.Kill(pid, 0)` from a product |
| the calling process's own pid, for a diagnostic record | `process.Self` — never `os.Getpid` from a product |
| walking the host's process table where the kernel offers a snapshot (Windows) | `process.ObserveProcesses` — never Toolhelp through `golang.org/x/sys` from a product; POSIX composes ps through `Run` |
| whether another process holds a file against opening (Windows share modes) | `filestore.ObserveSharing` — never a `CreateFile` probe from a product; POSIX composes lsof through Process |
| whether a path still names the file you hold, before removing or trusting it | `filestore.ObserveHeldStanding` — never `os.SameFile` over a raw stat pair from a product; absence is an observation, and the moment after the answer belongs to the advisory lock |
| the path of the binary running right now, for a supervisor unit | `process.Executable` — never `os.Executable` from a product |
| this process's inherited environment, to filter before handing to a child | `process.AmbientEnvironment` — never `os.Environ` from a product |
| one exact ambient environment variable without materializing the complete environment | `process.LookupAmbientEnvironment` — never `os.Getenv` or `os.LookupEnv` from a product; typed presence distinguishes absent from present-empty |
| ending a contained tree's survivors, even after the leader was reaped | `process.Execution.Sweep` — never `syscall.Kill(-pid, ...)` from a product; a group already gone is success, and a direct child is refused |
| the current working directory as a typed path | `process.WorkingDirectory` — never `os.Getwd` then re-parse |
| hashing a stream you are already moving | `core.DigestWriter` — an `io.Writer` that also peeks its running total mid-stream (`Digest`) and clears for pooled reuse across streams (`Reset`); never a private `sha256.New()` accumulator |
| hashing one whole in-memory buffer | `core.SHA256Of` for a typed digest, `core.SHA256BytesOf` for the raw thirty two bytes — never `sha256.Sum256` from a product, and never a fallible `Bytes` on a freshly hashed buffer, which is a branch no caller can exercise |
| naming a sibling by suffix (`.lease`, `-quarantine`) | `AbsolutePath.WithSuffix` — never string concatenation, which can leave the directory |
| a path relative to a root you hold | `AbsolutePath.RelativeTo` — never `filepath.Rel` then re-parse |
| building a nested path | `AbsolutePath.Resolve(names...)`, or `Join` / `JoinRelative` for typed parts — never `filepath.Join` then re-parse |
| an error identity | `core/error_identity.go` |

## Keeping this honest

`TestCapabilitiesDocumentDescribesEveryPackage` reads the package directories on
disk and checks every one has a described row above. Adding a package without
describing it here fails the build, on purpose: a capability nobody can find is
a capability somebody rebuilds. There is no list of package names in the test,
so there is no second copy of the answer to drift.
