# Primitive Policy

Primitive owns bounded, typed real-world capabilities and shared low-level
contracts. Product repositories own policy, persistence, workflow, and state
transitions. This projection is ratcheted against the compiler-owned catalog.

## 15. Exact package graph

| Order | Package | Purpose | Production imports | Test-only imports |
| --- | --- | --- | --- | --- |
| 1 | `core` | Shared nominal values, errors, paths, protocol facts, numeric and encoding contracts | none | none |
| 2 | `attest` | Canonical Ed25519 envelopes and proof-carrying verification | `core` | none |
| 3 | `contextstate` | Nil-safe context ingress and terminal observation | `core` | none |
| 4 | `currency` | Exact minor-unit values, arithmetic, ordering, and decimal projection | `core` | none |
| 5 | `keygen` | Exact secret and Ed25519 key generation | `core` | none |
| 6 | `testserial` | Test-only isolation declaration and analyzer contract | `core` | none |
| 7 | `filelock` | One advisory whole-file lock on one already-open file | `core`, `contextstate` | none |
| 8 | `filestore` | Rooted OS handles, confinement, inspection, durability, activation, append rotation, rename, and recovery | `core`, `contextstate`, `temporal` | `filelock` |
| 9 | `hostfacts` | Host disk, memory, cgroup, tree, and OOM observations | `core`, `contextstate` | none |
| 10 | `temporal` | Time, duration, arithmetic, persistence, waits, and tickers | `core`, `contextstate` | none |
| 4 | `exchange` | Bounded client and server boundary policy over `net/http` | `core`, `contextstate`, `keygen`, `temporal` | none |
| 12 | `fuzzfinder` | Bounded classification and observation of Go-generated fuzz artifacts | `core`, `filestore` | none |
| 13 | `lease` | Signed lease timeline, assessment, renewal, and monotonic advance | `core`, `temporal`, `attest` | none |
| 5 | `gate` | Pure CLI-side new-work authorization over one authentic Lease assessment | `core`, `lease` | `attest`, `temporal` |
| 15 | `receipt` | Authenticated accepted-evidence facts and fixed-size monotonic watermarks | `core`, `attest`, `temporal` | none |
| 16 | `controlwire` | Shared control-wire facts and paired authenticated socket with request-owner body limits | `core`, `keygen`, `exchange`, `temporal` | `controlplane`, `controlplanetest`, `attest` |
| 17 | `controlplane` | Signed control-plane request and response documents, their binding to one exact request, product status, and usage watermark | `core`, `controlwire`, `attest`, `lease`, `temporal`, `receipt` | none |
| 18 | `submission` | Authenticated evidence declarations, authority upload grants, and device-signed provider completion evidence bound to one exact request | `core`, `attest`, `chit`, `controlwire`, `id`, `objectstore`, `temporal`, `receipt` | `exchange` |
| 19 | `submissionauth` | Installation-certificate binding, device authentication, and authority reconciliation for evidence submissions | `core`, `attest`, `controlplane`, `controlwire`, `submission`, `chit`, `objectstore`, `receipt` | `controlplanetest`, `exchange` |
| 20 | `controlplanetest` | Real authority-signed installation certificate fixtures for hostile control-plane tests | `core`, `attest`, `controlplane`, `controlwire`, `lease`, `receipt`, `temporal` | none |
| 21 | `process` | Argv, environment, containment, bounded output, exit, and reaping over `os/exec` | `core`, `contextstate`, `temporal` | `testserial` |
| 22 | `release` | Clean repository binding, verified Go builds and process plans, bounded maintainer material exchange, executable inspection, signed tool and metadata provenance, immutable artifacts, manifests, Latest, and selection | `core`, `temporal`, `attest`, `filestore`, `controlwire`, `keygen`, `process` | `testserial` |
| 23 | `shutdown` | Signal observation and phased bounded cleanup | `core`, `contextstate`, `temporal` | none |
| 24 | `objectstore` | Bounded vendor-specified S3, GCS, or Cloudflare Images transfers through issued HTTPS capabilities, with integrity and provider evidence | `core`, `contextstate`, `temporal`, `exchange` | none |
| 25 | `timeproof` | RFC 3161 request construction, response verification, and replay | `core`, `temporal`, `keygen` | none |
| 26 | `cloudidentity` | Bounded Google Cloud identity-token and OAuth access-token or AWS identity-token acquisition with redacted disclosure | `core`, `contextstate`, `temporal`, `exchange` | none |
| 27 | `deploy` | Exact create-only GCS publication of one authenticated release and its metadata | `core`, `objectstore`, `release` | `attest`, `exchange`, `temporal` |
| 28 | `upgrade` | Crash-recoverable installation, activation, startup truth, rollback, and recovery | `core`, `filestore`, `hostfacts`, `objectstore`, `release`, `temporal` | `exchange` |
| 29 | `gcsobjects` | Authenticated Google Cloud Storage bucket provisioning and public-read IAM, typed logical namespace composition, create-only writes, IAM-signed short-lived upload and whole-object retrieval capabilities, exact-generation observation, digest-bound reads, and generation-matched permanent deletion through official SDKs over Exchange | `core`, `contextstate`, `temporal`, `objectstore`, `exchange`, `filestore` | `testserial` |
| 30 | `id` | Standard-library-backed UUIDv7 and canonical ULID time-ordered identifiers from one observed instant and caller-supplied entropy | `core`, `temporal` | none |
| 31 | `chit` | Authority-signed immutable custody tickets, streaming manifest closure, bounded catalogs, and device-signed catalog queries | `attest`, `core`, `controlwire`, `id`, `receipt`, `temporal` | none |
| 32 | `chitauth` | Installation-certificate binding and device authentication for one chit catalog query | `chit`, `controlplane`, `controlwire`, `core` | `controlplanetest`, `attest`, `receipt`, `temporal` |
| 33 | `retrieval` | Device-signed exact-object requests, authority-signed expiring download capabilities bound to authenticated chit manifests, and atomic exact-file retrieval execution | `attest`, `chit`, `controlwire`, `core`, `filestore`, `objectstore`, `temporal` | `exchange`, `receipt` |
| 34 | `retrievalauth` | Installation-certificate binding and device authentication for one evidence-retrieval request | `attest`, `controlplane`, `controlwire`, `core`, `retrieval` | `controlplanetest` |
| 35 | `payment` | Authority-signed exact payment receipts, bounded catalogs, and device-signed catalog queries | `attest`, `core`, `controlwire`, `currency`, `id`, `receipt`, `temporal` | none |
| 36 | `paymentauth` | Installation-certificate binding and device authentication for one payment catalog query | `controlplane`, `controlwire`, `core`, `payment` | `controlplanetest`, `attest`, `currency`, `receipt`, `temporal` |
| 37 | `distribution` | Signed product-neutral release publication, update discovery, and exact upgrade-download agreements | `attest`, `controlwire`, `core`, `deploy`, `objectstore`, `release`, `temporal`, `upgrade` | `exchange` |
| 38 | `distributionauth` | Authenticated release-material responses plus installation-certificate binding and device authentication for publication, update, and upgrade requests | `attest`, `controlplane`, `controlwire`, `core`, `distribution`, `release` | `controlplanetest`, `deploy`, `exchange`, `objectstore` |
| 39 | `wiring` | Bounded immutable runtime component graphs with exact Primitive-door declarations | `core` | none |
| 40 | `lineio` | Bounded line scanning over one `io.Reader` through Go `bufio.Scanner` and `bufio.ScanLines` | `core` | `filestore` |
| 41 | `manual` | Bounded validated human text and stable machine JSON manuals from one product-owned typed book | `core` | none |
| 42 | `secretstore` | Bounded exact-version secret access through official provider SDKs | `core`, `contextstate` | `process` |
| 43 | `projectstandards` | Validated project and package knowledge, exact evidence references, deterministic reports, and bounded exchange | `core`, `exchange`, `id`, `temporal` | none |
| 44 | `machineprobe` | Bounded execution and typed evidence capture for one admitted machine-observation script | `projectstandards`, `core`, `filestore`, `process`, `temporal` | `id` |
| 45 | `runnercontrol` | Typed domain-blind runner admission, execution, evidence, completion, and delivery contracts | `projectstandards`, `attest`, `core`, `exchange`, `id`, `objectstore`, `process`, `temporal` | none |
| 46 | `runworkspace` | Owned per-run writable workspace, source acquisition, evidence retention, and cleanup effects | `projectstandards`, `attest`, `core`, `filestore`, `objectstore`, `process`, `runnercontrol`, `temporal` | `exchange`, `id` |
