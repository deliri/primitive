# `cmd/keygen` command interview

Status: `COMPLETE` | Decision: `DEFER`

This is the exclusive reconstruction report for the archived command package
`cmd/keygen`. The Keygen library has its own report in `keygen.md`; this report
examines only the human/process boundary that selects material, generates it,
writes one private file, reports public facts, and classifies exit.

The archive and all four consumer repositories were read-only. The only
source-tree write made by this interview is this report.

The provisional result has three parts:

- The archive contains strong reusable command mechanics: real-binary process
  tests, exact canonical file verification through Core, create-only durable
  Filestore composition, preservation of existing filesystem state, recovery
  attempts, and a meaningful private-material disclosure oracle.
- The command is not admissible unchanged. It silently accepts ambiguous
  invocations, ignores terminal write errors, copies clearable private bytes
  into immutable strings, has no typed command request or local flag parser,
  uses an identity-blind path deletion for post-activation cleanup, and relies
  on an exact `0600` guarantee Filestore does not establish under hostile umask
  or native Windows semantics.
- No inspected consumer invokes this executable or needs its exact
  one-material-file workflow. Kernel, Witness, Bug, and Peachfuzz each own a
  materially different product lifecycle. Absence alone is not a retirement
  proof, but Primitive 2026 should not admit a security-sensitive executable
  without an identified operator, installation path, and end-to-end custody
  workflow.

The implementation therefore remains unready: preserve its best mechanics as
requirements, correct the verified defects if a real generic workflow sponsors
it, and otherwise retire the executable while retaining the Keygen library.

## Evidence boundary


| Source | Exact revision and Primitive pin | `cmd/keygen` tree | Working-tree qualification |
| --- | --- | --- | --- |
| Archived Primitive | HEAD `d046f7b675fcb797398d7cdc87b5504f43978056` (`2026-07-27T03:35`, `2026-07-27T03:41-04`, `2026-07-27T03:00`) | `4d01d76f0524301811882662762265176fce7942` | One unrelated pre-existing untracked file, `core/api_http_boundary_hostile_test.go`; no archive file changed during this interview. |
| Kernel | HEAD `fec28ef7c9c0ab7e31bfa72127053f96deefcb59`; committed `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:go.mod:76` pins `0df2954a2d911a5d7d775691d023d569affa2c20`; dirty `kernel@working-tree:go.mod:76` pins `e8b7172161a4994efcb7f092113e23c28928da43` | Committed-pin tree `331d923a9bf42e37c98a2edaa533865b972c53ac`; dirty pin is the exact archive tree `4d01d76f0524301811882662762265176fce7942`. | Materially dirty tree, including the Primitive pin and unrelated product work. Kernel has its own product command; it does not execute the Primitive command. |
| Witness | HEAD `b9629af57b7058b68982be5d3b282be440b1e76e`; `witness@b9629af57b7058b68982be5d3b282be440b1e76e:go.mod:17` pins `773add8ba0fc1a9453cc06c8558b8541c1fc8ce9` | `19099c27100bc18231d1f1b99e9ba193905d9f40` | Only untracked `.ledger_pending.md`; tracked source and module pin match HEAD. No command invocation found. |
| Bug | HEAD `39ce96242240d7174d562c90bb255860946595dc`; `bug@39ce96242240d7174d562c90bb255860946595dc:go.mod:9` pins `388e593231a28434f6faae9f0ab9dffcf332dfc3` | `34d2a4b38040e318c8947c166f5219bda1bb05c9` | Only untracked `.ledger_pending.md`; tracked source and module pin match HEAD. No command invocation found. |
| Peachfuzz | HEAD `2b2d080c455edaadf88502c1c253845605a4336a`; `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:go.mod:5` pins `3f74d8fc35b4f0f1ddd65ec0e626ee1e06060d75` | `331d923a9bf42e37c98a2edaa533865b972c53ac` | `.ledger_pending.md` is modified; production and test sources inspected here otherwise match HEAD. No command invocation found. |

Every consumer module pin contains a historical copy of the command because it
pins the whole Primitive module. Requiring a Go module does not build or run
its `main` packages. The pins are exact version evidence, not evidence of
command adoption.

The final archived package contains six files and 1,449 lines:

| File | Lines | Role |
| --- | ---: | --- |
| `SPEC.md` | 12 | Minimal command description |
| `command_process_hostile_test.go` | 701 | Built-process argument, output, exit, disclosure, and filesystem tests |
| `doctrine_contract.go` | 7 | Doctrine witness |
| `main.go` | 182 | Flag dispatch, generation, persistence, recovery, and cleanup |
| `main_test.go` | 139 | Direct Filestore boundary smoke tests |
| `private_material_hostile_test.go` | 408 | Path resolution and cleanup unit proof |

The committed Kernel and Peachfuzz pins contain an older 138-line command. It:

- accepts kinds `ed25519`, `secret`, and `garble`;
- parses a Core-owned `KeygenKind`;
- calls the old Keygen library API;
- writes directly with `os.OpenFile(O_CREATE|O_EXCL)`;
- syncs the file but not the containing directory; and
- removes the output path directly after write/sync/close failure.

See `archive@0df2954a2d911a5d7d775691d023d569affa2c20:cmd/keygen/main.go:1-138`.

The final archive renames the kinds to `signing`, `secret`, and `custody`,
switches to the final Keygen value API, and delegates create-only activation,
file and directory durability, symlink refusal, recovery, and removal to
Filestore (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/keygen/main.go:1-182`).

The source drift is substantial:

| Consumer baseline | Diff from its command tree to archived HEAD |
| --- | --- |
| Kernel committed pin | 5 files, 1,289 insertions, 67 deletions |
| Witness pin | 5 files, 1,289 insertions, 67 deletions |
| Bug pin | 6 files, 1,291 insertions, 67 deletions |
| Peachfuzz pin | 5 files, 1,289 insertions, 67 deletions |

No compatibility flags or old kind aliases should be restored. If the command
survives, it needs one clean 2026 CLI contract and direct workflow documentation.

The command history shows rapid hardening:

1. `e499d06fdf3fb903cb41019d9cb3b1a1510fe374` introduced Ed25519 and
   symmetric generation on 2026-07-18.
2. `2afba21dc342156b554a83ff14843d87cae5fc68`,
   `ec86e85c75bca455e6606db2927db557be472f65`, and
   `627f293cc4a1ac6790c81ec6c12ee0a2a75feb08` hardened Keygen and added
   custody that same day.
3. `ec71e0e3fdbc432f1b1051589b2c3225dc34a6e6` changed output to exact private
   material bytes.
4. `4a57cd1e808c843b4fe312386600bfba9bb37125`,
   `c11c22e53ab6c6cef1b4cd70c1e67620c7e58151`, and
   `8c20a20138919f725269dd5a4d820bdf7081b77e` evolved shared types and
   consumer contracts through 2026-07-22.
5. `cf8fa882ee56bb263220a1466c6deae08e466018` moved the command to the final
   primitive boundaries.
6. `d259789e87bcadb829c5ffac72c6c91ccc604098` centralized CLI facts and
   error identity.
7. `c3f9b008551570db8a422839ce7fad49ed3979e0` added the large hostile process
   and cleanup suites on 2026-07-26.
8. `40ded9c104a99cbc4b0b672cd7392901b468d1eb` added the final comparative
   contract record later that day.

The archive's own freeze board still classified `cmd/keygen` as `Open`, noting
that current and copied commands needed indexing
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:_docs/governance/foundation_capability_freeze_audit.md:510-535`).
The earlier comparative audit's statement that the command was accepted is
therefore a lead, not final admission evidence.

## Capability ownership

The command composes two lower capabilities:

```text
core contracts ------+
keygen generation ---+--> cmd/keygen
filestore mutation --+
```

It currently owns:

1. parsing `-kind`, `-out`, and `-bytes` from the process argument vector;
2. dispatching to one of three Keygen operations;
3. explicitly marshaling private material through its Core value;
4. choosing a caller-named destination and private-file policy;
5. writing create-only through Filestore;
6. attempting Filestore recovery and closing an unresolved capability;
7. deleting an activated output after a failed operation;
8. printing a success message and, for signing material, the public key;
9. keeping canonical private text off stdout and stderr; and
10. mapping malformed invocation, execution failure, and success to exit 2, 1,
    and 0.

See `archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/keygen/main.go:24-182`.

Paraphrased, the specification intends a narrower boundary: the executable
projects Keygen plus Filestore, accepts one closed material kind and validated
destination, creates one private file, reports only public signing material,
preserves existing state, and cleans only its owned path.

See `archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/keygen/SPEC.md:3-12`.

That is the right general category. A command should compose capability owners,
not reopen cryptography or filesystem mutation. It should not own algorithms,
secret derivation, product env schemas, key rotation, secret-manager policy,
installation identity, or application startup recovery.

The truthful candidate capability for 2026 is:

> An optional offline operator tool that creates exactly one new canonical
> private-material file from one typed Keygen request, reports only explicitly
> public facts, never overwrites or follows links, and returns an exit code that
> truthfully reflects parsing, generation, durable activation, cleanup, and
> terminal output.

The word **optional** is load-bearing. None of the inspected product workflows
uses this tool, so the executable must not become a mandatory provisioning
layer merely because the archive contains it.

## Archive evidence

### Archived mechanics worth retaining

### Lower-capability composition

The final command imports `core`, `filestore`, and the Keygen library. It does
not call `crypto/rand`, `ed25519.GenerateKey`, `os.OpenFile`, `os.WriteFile`,
`os.Remove`, `os.Rename`, or `os.Chmod` in production.

Keygen creates typed material; Core owns canonical encoding; Filestore owns
root-confined create-only persistence, activation, synchronization, recovery,
and removal (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/keygen/main.go:19-22`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/keygen/main.go:53-182`).

This is much stronger than the pinned command, which directly opened, wrote,
synced, closed, and removed a raw path
(`archive@0df2954a2d911a5d7d775691d023d569affa2c20:cmd/keygen/main.go:99-138`).

### Explicit private-material crossing

Each generation branch:

- obtains a validated Core value;
- calls its explicit `MarshalText` boundary;
- schedules the mutable returned byte slice for clearing;
- writes only that canonical representation; and
- emits no private value in its success message.

See `archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/keygen/main.go:53-103`.

For signing material, the public key printed to stdout is derived from the same
pair whose private form was written (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/keygen/main.go:70-85`).
The process test independently parses the file through
`core.ParseEd25519KeyPair` and requires stdout to contain that derived public
key (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/keygen/command_process_hostile_test.go:93-115`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/keygen/command_process_hostile_test.go:270-313`).

That file-to-public-output linkage is strong, non-duplicated evidence.

### Real process-path proof

`TestMain` builds the actual command once. The process helpers execute that
binary with a private working directory and capture its real stdout, stderr,
and exit status (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/keygen/command_process_hostile_test.go:20-91`).

The accepted-generation table drives the full path:

```text
argv -> flag.Parse -> dispatch -> Keygen -> Core marshal
     -> Filestore write/commit -> real file -> terminal -> process exit
```

It then proves:

- exit zero;
- exact `0600` mode under the native test environment;
- Core acceptance of the written file;
- exact secret width;
- public/private correspondence;
- absence of a 16-character private fragment on both terminal streams; and
- no extra directory entry.

See `archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/keygen/command_process_hostile_test.go:146-314`.

This is genuine command production-path proof, not a helper standing in for
`main`.

### Create-only preservation

Filestore uses `InstallCreate`; the command never overwrites an existing path
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/keygen/main.go:105-125`).

Direct tests cover an existing permissive file, a symlink output, a directory
output, and a symlinked parent
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/keygen/main_test.go:13-139`).

The real-process preservation table expands that to:

- existing owner-only and world-readable regular files;
- an empty file;
- a symlink to an operator file;
- a dangling symlink;
- empty and non-empty directories; and
- a symlinked parent.

It snapshots the whole test directory before and after and requires byte-for-byte
and mode-for-mode preservation
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/keygen/command_process_hostile_test.go:528-701`).

### Bounded canonical write

The write ceiling is exactly the length of the already bounded canonical
material. The Keygen library bounds raw generic material to 64 bytes, and the
Ed25519 private representation is fixed. No command path reads an unbounded
stream or accumulates history (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/keygen/main.go:113-125`).

The file contains no trailing newline or presentation wrapper. Core parsing of
the exact bytes is the oracle.

### Recovery and original-cause preservation

After `filestore.Write`, the command:

- observes whether the outcome retains recovery;
- attempts recovery;
- closes the remaining capability;
- joins write, recovery, and close errors;
- distinguishes pre-activation from activated failure; and
- tries to remove material only after activation may have occurred.

See `archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/keygen/main.go:126-145`.

The cleanup unit table proves the initiating error survives successful cleanup,
an absent target does not fabricate an error, and cleanup failures join rather
than replace the original cause
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/keygen/private_material_hostile_test.go:179-324`).

Separate tests prove symlink cleanup unlinks the link without following it and
does not remove neighboring files
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/keygen/private_material_hostile_test.go:326-383`).

### Typed lower-level error identity

Every private-material persistence failure joins
`core.ErrKeygenCLIPrivateMaterial`; underlying Filestore, filesystem, and
original errors remain discoverable with `errors.Is`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/error_identity.go:22-24`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/keygen/main.go:147-182`).

Direct tests assert the Core identity and `fs.ErrExist`, while cleanup tests
assert both the initiating sentinel and cleanup identity
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/keygen/main_test.go:42-83`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/keygen/private_material_hostile_test.go:287-323`).

### Primitive-internal use and workflow status

No Primitive library imports a `main` package, and no archived Primitive
production path invokes `cmd/keygen` as a child process.

No build, install, release, or shell script found in the archive packages this
binary. The only located out-of-package references are:

- governance records;
- the root package inventory; and
- Core constants/error identities used by the command.

The command is therefore an unintegrated human tool at archived HEAD. Its tests
prove that `go run`/a built binary can work, but the archive does not establish:

- how an operator obtains the binary;
- which workflow asks the operator to run it;
- which component consumes each output form;
- where the private file should move after generation;
- how rotation, revocation, or deletion completes; or
- whether a generic naked private file is preferable to product-owned
  create-and-persist workflows.

The header claims that the output is the exact canonical form accepted by
Primitive ingress, but no actual ingress command or service is cited
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/keygen/main.go:1-7`). Parsing the file back through Core proves
canonical shape, not operational adoption.

## Consumer evidence

### Kernel: similarly named, materially different product command

Kernel has its own `cmd/keygen`; it does not invoke Primitive's command.

Kernel's standalone command:

- accepts only `-force`;
- calls product `keygen.EnsureAll("env", force)`;
- provisions six named environment values across `dev.env`, `stage.env`, and
  `prod.env`; and
- preserves non-empty values unless force is explicit.

See `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:cmd/keygen/main.go:1-102` and
`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:cmd/keygen/keygen.md:1-27`.

Kernel's main `lfw keygen` workflow first synchronizes product env files while
preserving App Engine-specific fields, then provisions keys without force
(`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:cmd/lfw/keys.go:76-105`). Its keycheck diagnostic explicitly directs
operators to `go run ./cmd/lfw keygen`
(`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:cmd/lfw/keys.go:257-297`).

This workflow is not replaceable by a command that emits one anonymous private
file. Product env names, linked Ed25519 halves, shared-database pepper/HMAC
reconciliation, force policy, and environment sync remain Kernel-owned.

**Local gem for Primitive's command shell.** Kernel uses:

- a local `flag.FlagSet` with `ContinueOnError`;
- injected stdout, stderr, and operation function;
- a validated typed `runInput`;
- one `runWithInput` that returns an exit code instead of calling `os.Exit`;
- tests that prove invalid wiring short-circuits before side effects.

See `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:cmd/keygen/main.go:22-102` and
`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:cmd/keygen/main_test.go:17-160`.

That testable imperative-shell pattern is product-neutral and stronger than the
archive command's global `flag.CommandLine` and direct `fmt.Println` calls.
Primitive should adopt the mechanic, not Kernel's env workflow or constants.

The comparative audit states that `Kernel's copied key generator is retired`.
That statement is too broad if read as deleting Kernel's command
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:_docs/governance/protocol_capability_comparative_audit.md:142-145`).
Kernel's raw generation primitive may retire after library migration; its env
composition remains product policy.

### Witness: setup-owned local identity, public-only projection tool

Witness does not invoke Primitive `cmd/keygen`.

Its setup path creates a project-local trust key automatically through
`attest.EnsureProjectLocalTrustKey`
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:cmd/witness/setup.go:80-107`). The owning implementation:

- refuses to overwrite an existing private key;
- derives or repairs the public file from the private file;
- rejects an orphan public file;
- encodes the private key as OpenSSH material; and
- publishes private and public files with separate durable policies
  (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/attest/local_key.go:19-45`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/attest/local_key.go:88-170`).

Witness also has `cmd/witness-sign`, but that command does not generate or
persist private material. It loads a configured private signer and prints only
the hex public key
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:cmd/witness-sign/main.go:1-74`).

**Local gem.** Witness couples key creation to the installation trust-mode
lifecycle and keeps the public projection as a separate, public-only tool.
No-overwrite, private-to-public repair, and OpenSSH encoding are valid product
requirements. They remain downstream and demonstrate why a generic Primitive
private file cannot be the only provisioning workflow.

### Bug: automatic device identity, no naked-key command

Bug does not invoke Primitive `cmd/keygen`.

During license/device setup, Bug creates a `WriterPrivateSeed`, derives the
writer public identity and device fingerprint, validates their relationship,
and persists them as one product `Device` record
(`bug@39ce96242240d7174d562c90bb255860946595dc:internal/license/device.go:16-42`, `bug@39ce96242240d7174d562c90bb255860946595dc:internal/license/device.go:64-145`,
`bug@39ce96242240d7174d562c90bb255860946595dc:internal/license/writer_key.go:17-91`).

The CLI creates the device when the license workflow finds no existing device
(`bug@39ce96242240d7174d562c90bb255860946595dc:cli/license.go:850-885`).

**Local gem.** Bug's private seed never exists as an anonymous operator file.
It is born inside the typed device lifecycle, immediately tied to its public
identity and product persistence record. Primitive Keygen may eventually
supply the random material, but Primitive's generic command must not replace
the product transaction, label, fingerprint, signing domain, or durable record.

### Peachfuzz: daemon-owned load-or-create identity

Peachfuzz does not invoke Primitive `cmd/keygen`.

At daemon startup it:

- checks for a retired unsigned identity and fails loudly if present;
- reads and strictly decodes the existing machine-evidence identity;
- generates a new identity only on `fs.ErrNotExist`;
- persists the full typed identity create-only; and
- never silently rekeys an installation.

See `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/app/daemon_identity.go:19-84` and
`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:README.md:174-191`.

The actual generation stays inside Peachfuzz's Professor crypto boundary
(`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/professor/run_evidence.go:9-17`).

**Local gem.** Peachfuzz binds creation, persistence, machine namespace, and
no-rekey recovery into one load-or-create transaction. This is stronger for its
workflow than asking an operator to generate a detached file and manually
install it. That lifecycle remains downstream.

## Strong mechanics and proof

### Testing-protocol proof

The project-local `_docs/testing_protocol.md` was read completely before this
interview. The command tests were assessed against it, not merely against green
tool output.

| Protocol contract | Archived proof | Assessment |
| --- | --- | --- |
| `test/evidence` and `test/production-path` (`foundation@working-tree:_docs/testing_protocol.md:149-170`, `foundation@working-tree:_docs/testing_protocol.md:862-891`) | The built executable drives actual flags, Keygen, Filestore, disk, terminal, and exit (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/keygen/command_process_hostile_test.go:20-314`) | Strong for ordinary success/refusal. Post-activation fault cleanup is only a direct helper test, not production-path proof. |
| `test/isolation/tempdir` (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/keygen/command_process_hostile_test.go:228-275`) | Every parallel filesystem subtest owns `t.TempDir`; `TestMain` manually owns and removes its binary directory | Strong for test units. |
| `test/parallel/default` (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/keygen/command_process_hostile_test.go:277-305`) | Parents and subtests consistently call `t.Parallel()` | Strong. |
| `test/determinism` (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/keygen/command_process_hostile_test.go:356-380`) | Paths and process streams are controlled; random material is parsed for invariant facts rather than compared to fixed bytes | Strong for outcomes. Real CSPRNG use makes exact private content intentionally nondeterministic. |
| `test/table-shape` and `test/boundaries` (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/keygen/command_process_hostile_test.go:382-516`) | 17 accepted invocations, 23 malformed/refused invocations, 8 existing-state cases, 18 path cases, and 7 cleanup cases | Quantitatively strong. Three accepted cases ratchet unsafe ambiguity instead of rejection. |
| `test/errors` (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/keygen/command_process_hostile_test.go:644-696`) | Direct helpers use `errors.Is` for Core, Filestore, filesystem, and initiating identities | Strong locally. Process tests can only assert exit and diagnostic because errors terminate at the process boundary. |
| `test/fixtures/non-vacuous` (`foundation@working-tree:_docs/testing_protocol.md:837-860`) | Success cases require one file; refusal cases require no directory entries or an identical whole-tree snapshot | Strong. |
| `test/structural-invariant` (`foundation@working-tree:_docs/testing_protocol.md:1008-1044`) | No command-specific import, effect, kind-token, or exit-shape ratchet | Missing. The spec's Keygen-plus-Filestore ownership can drift without a command-local build failure. |
| `test/data-flow-inventory` (`foundation@working-tree:_docs/testing_protocol.md:1046-1099`) | Production declares no typed request/result structs; raw flag pointers and strings carry the trust-boundary state | Missing compiler-visible command request and inventory. |
| `test/fuzz-boundary` (`foundation@working-tree:_docs/testing_protocol.md:1210-1288`) | No CLI argument fuzzer | Gap. CLI parsing is an explicitly recommended fuzz boundary; hostile tables are strong but do not explore conditional/repeated/positional flag combinations generally. |
| Layer triad / evidence path / ledger rules | The command writes private state, not an evidence bundle or accounting ledger | Evidence/ledger triads are truthfully `NOT_APPLICABLE`. The process tables nevertheless contain success, refusal, and help/no-op cases. |
| Benchmarks | At most a fixed key or 64-byte secret is handled | Truthfully `NOT_APPLICABLE`; no performance claim needs a benchmark. |

### Reproduced archive-local gates

At archived HEAD:

- `go test ./cmd/keygen -count=1`: PASS;
- `go test -race ./cmd/keygen -count=1`: PASS;
- `go test ./cmd/keygen -shuffle=on -count=10`: PASS;
- `go test -cover ./cmd/keygen -count=1`: PASS, 32.9% statements;
- `go vet ./cmd/keygen`: PASS;
- `staticcheck ./cmd/keygen`: PASS;
- production `gocyclo -over 10 cmd/keygen/main.go`: PASS;
- Linux amd64 test-binary cross-build: PASS;
- Windows amd64 test-binary cross-build: PASS;
- Darwin arm64 test-binary cross-build: PASS.

The low coverage percentage needs interpretation: the strongest tests execute a
separately built uninstrumented command, so their production statements do not
appear in the package coverage profile. It is not proof of missing process
execution, but it also means the coverage number cannot ratchet those paths.

Whole-directory `gocyclo` reports complexity above 10 in five test functions
(26, 16, 11, 11, and 11). The global complexity requirement applies to
production, which is clean; the test functions would still be easier to audit
if their case executors were split without hiding assertions.

Cross-build success proves compilation only. The symlink and permission tests
were run natively on Darwin, not on Linux and Windows.

## Defects and blockers

### B1: success-output failures are ignored

Every success branch ignores the `(count, error)` returned by `fmt.Println` or
`fmt.Printf`:

- custody announcement: `archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/keygen/main.go:53-67`;
- signing announcement and public key: `archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/keygen/main.go:70-85`;
- secret announcement: `archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/keygen/main.go:88-102`.

If stdout is a closed pipe or failing descriptor, the private file may be
durably created while the command returns exit 0 even though its public/output
contract did not complete. The signing case is particularly important: the
operator can lose the advertised public key while the command claims success.

The process tests always attach healthy `bytes.Buffer` streams and cannot make
this path red (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/keygen/command_process_hostile_test.go:71-91`).

Primitive 2026 needs an injected command shell whose output errors are
observed. The spec must decide the post-durability outcome: a terminal-reporting
failure should return nonzero but must not casually delete a successfully
durable private key merely because stdout broke. The typed result must tell the
operator that the file exists and public projection failed.

### B2: private material is copied into an immutable string

Core returns private text as a mutable `[]byte`, and each branch correctly
schedules that slice for clearing. It then immediately converts the slice to a
`string`:

```go
writePrivateMaterial(path, string(material))
```

See `archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/keygen/main.go:58-64`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/keygen/main.go:75-80`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/keygen/main.go:93-99`.

`writePrivateMaterial` accepts the string and passes it to
`strings.NewReader` (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/keygen/main.go:108-123`). The string copy
cannot be cleared. Clearing the original byte slice does not clear the
immutable copy, and the command's strong private-material handling becomes
partly theatrical.

The older command already carried private strings in generated structs and
`io.WriteString`; the final implementation did not retire that memory shape.

The 2026 write path should accept a bounded `[]byte` or a private-material
reader that preserves explicit ownership, use `bytes.NewReader`, and clear the
owned byte slice after Filestore completes. The command must not log, format,
or copy it into a Go string.

### B3: ambiguous invocations are deliberately accepted

The accepted process table explicitly pins:

- `-bytes` on `signing` as silently ignored;
- `-bytes` on `custody` as silently ignored;
- repeated `-kind` with the final value winning; and
- trailing positional arguments as discarded.

See `archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/keygen/command_process_hostile_test.go:162-184`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/keygen/command_process_hostile_test.go:248-268`.

For a private-key generator, ambiguity is unsafe. A caller that appends a
second kind can receive a different material class; a typo after valid flags
still mints a key; and an irrelevant width flag can make an automation author
believe a constraint was honored when it was not.

These tests ratchet accidental `flag` behavior instead of the intended closed
request. Admission requires red rejection cases for:

- every repeated singleton flag;
- every positional argument;
- `-bytes` unless kind is `secret`;
- missing, empty, unknown, or future kinds; and
- every out-of-domain width.

The parser should close into a typed command request and call `Validate()`
before generation.

### B4: invocation failure classification is inconsistent and string-owned

Lexically invalid widths (`-1`, `abc`) exit 2, while numerically parsed but
out-of-domain widths (zero, one, below minimum, above maximum, `MaxInt64`,
`MaxUint64`) exit 1
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/keygen/command_process_hostile_test.go:326-448`).

Both are invalid operator requests discovered before any legitimate generation
effect. The distinction comes from where the raw `flag.Uint64` failure happens,
not from a typed command policy.

The command also compares raw Core-owned strings in its switch and calls
`os.Exit(1|2)` with magic values
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/keygen/main.go:24-50`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/keygencli_constants.go:5-10`).

The comparative audit says the command owns `typed flag dispatch` and exit
classification, but the final implementation has neither a typed kind nor a
typed exit value
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:_docs/governance/protocol_capability_comparative_audit.md:142-145`).

Primitive 2026 should own a private closed `materialKind`, typed
`commandRequest`, and typed `exitCode`. Raw CLI tokens parse exactly once.
Invalid requests consistently map to usage/refusal; generation, persistence,
and terminal failures map to execution failure.

### B5: cleanup owns a path, not the activated file identity

After an activated write fails, the command calls:

```go
filestore.RemoveFile(context.Background(),
    filestore.FileRemovalRequest{Root: root, Target: target})
```

See `archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/keygen/main.go:139-180`.

`RemoveFile` stats the current target and then removes that path. It does not
compare the current identity to the file activated by the command, and there is
a race between stat and unlink
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:filestore/remove.go:15-44`).

Another actor can replace the path after activation/failure and before cleanup.
The command can then delete an operator-owned replacement while claiming
cleanup removes only `the command-owned path`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/keygen/SPEC.md:3-8`).

The direct cleanup tests pre-create arbitrary files and pass only a root and
target; they demonstrate that cleanup cannot distinguish ownership
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/keygen/private_material_hostile_test.go:179-324`). The symlink
and neighboring-file tests are valuable but do not exercise replacement
between activation and cleanup.

This must be fixed at the Filestore boundary, not with another path precheck.
Filestore should retain an exact rollback/recovery capability bound to the
activated object identity and rooted directory handle, or the command must
truthfully leave the file in an explicit ambiguous state. `cmd/keygen` must not
reconstruct ownership from a path after releasing the write capability.

### B6: exact `0600` is not established

The spec promises a file created with mode `0600`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/keygen/SPEC.md:3-6`). The command passes
`core.KeygenCLIPrivateMaterialFileMode` to Filestore
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/keygen/main.go:113-123`).

On Unix, creation mode is filtered by process umask. A hostile restrictive
umask can produce `0400`, `0200`, or `0000`; the command does not verify or
correct the descriptor mode after creation. On Windows, Unix permission bits
do not provide the claimed owner-only ACL guarantee.

Tests assert exact mode only under the native test process's ordinary umask
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/keygen/main_test.go:16-40`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/keygen/command_process_hostile_test.go:270-313`). There is no
serial umask matrix, descriptor-owned correction proof, native Windows ACL
proof, or platform-specific spec.

The broader Filestore interview already identifies exact creation mode as an
unclosed package defect. This command inherits that blocker. It cannot claim
owner-only private storage until the owning filesystem capability can establish
and verify the relevant native protection.

### B7: recovery is unbounded and not connected to process cancellation

The write, recovery, and removal paths all use `context.Background()`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/keygen/main.go:108-145`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/keygen/main.go:169-180`).

The command:

- does not observe SIGINT/SIGTERM through a composition-owned context;
- has no operation deadline;
- gives cleanup no bounded detached recovery context; and
- can wait indefinitely inside filesystem synchronization or recovery.

The process harness likewise uses unbounded `exec.Command` for both the package
build and each child run
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/keygen/command_process_hostile_test.go:24-43`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/keygen/command_process_hostile_test.go:71-91`). A wedged
child can wedge the suite.

For 2026, the entrypoint should compose the approved shutdown/process and
temporal boundaries, carry one bounded operation context into Filestore, and
derive a bounded detached cleanup context when cancellation must not abandon
private residue. Process tests need `CommandContext` or an equivalent timeout
backstop.

### B8: terminal disclosure proof is useful but incomplete

The process oracle slides a 16-character window of the canonical private file
text across stdout plus stderr
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/keygen/command_process_hostile_test.go:57-60`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/keygen/command_process_hostile_test.go:117-131`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/keygen/command_process_hostile_test.go:291-307`).

That catches whole or partial canonical-text leakage. It does not catch:

- decoded raw bytes;
- a different encoding such as base64 versus hex;
- decimal/reflection formatting of private arrays;
- a shorter but still meaningful prefix; or
- structured diagnostic fields derived from the material.

The returned `core.Ed25519KeyPair` itself lacks generic formatting redaction,
as recorded in the Keygen library interview. The current command does not
format the pair, but the command's oracle would not catch a future `%+v`
decimal-array disclosure.

Admission requires the Core key-pair redaction fix plus command hostile tests
over raw, canonical, alternate encoded, and generic-format representations. The
oracle must continue checking both success and error terminal paths.

### B9: the command spec is far below the repository admission contract

`cmd/keygen/SPEC.md` is twelve lines. It does not define:

- exact flag grammar or duplicate/positional behavior;
- typed request and validation order;
- exact stdout/stderr grammar and output-error handling;
- error identities and exit mapping;
- canonical private formats;
- memory ownership and clearing;
- operation and cleanup contexts;
- Filestore activation/recovery outcomes;
- post-activation rollback identity;
- resource bounds;
- Unix/Windows platform behavior;
- signal behavior;
- required hostile tests;
- installation/distribution; or
- the real operator and consuming workflow.

See `archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/keygen/SPEC.md:1-12`.

Primitive's local `AGENTS.md` requires ownership, non-ownership, dependencies,
contracts, validation boundaries, errors, bounds, platform behavior, and
hostile proofs before implementation. The archive spec cannot serve as a 2026
admission spec.

### B10: command-only facts are misplaced in Core

The kind tokens and `0600` mode live in
`core/keygencli_constants.go`, but no Primitive package or inspected consumer
outside this command uses them
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/keygencli_constants.go:1-10`).

Compiler ownership does not mean every constant belongs in Core. These are
command-private input and persistence policy. They should move beside the typed
command request if the executable survives.

`core.ErrKeygenCLIPrivateMaterial` may remain Core-owned because the global
repository instruction requires stable cross-package error identity in Core
and the error joins Filestore identities. Diagnostic prose and flag tokens
should not.

### B11: no adopted operator workflow justifies the executable yet

No consumer invokes the command:

- Kernel owns multi-env provisioning and `lfw keygen`;
- Witness setup owns local OpenSSH trust identity;
- Bug license setup owns device/writer identity;
- Peachfuzz daemon startup owns machine-evidence identity.

No archived Primitive installer or release path publishes this binary, and no
located ingress consumes its output.

This is not automatic retirement:
`foundation@working-tree:PLAN.md:152-161` defers `cmd/keygen` and permits
re-entry with current demand, one product-neutral owner, its complete
dependency frontier, and explicit user approval. A human-facing command,
however, needs more than import demand. It needs a named operator journey and
custody handoff because it deliberately places private material on disk.

Before admission, one sponsor must establish:

1. who runs it;
2. how they obtain an authenticated binary;
3. which material kind they need;
4. which typed consumer accepts the file;
5. how the file moves to its final secret store;
6. how temporary private material is deleted and that deletion verified;
7. how rotation/revocation works; and
8. why product-owned automatic creation is not safer.

Without that workflow, retire `cmd/keygen` and keep the library.

## Primitive 2026 ownership and DAG

The command belongs in the top support/entrypoint layer:

```text
core -------------+
keygen -----------+
filestore --------+--> cmd/keygen
shutdown/process -+
temporal/context -+
```

No library imports the command. The command consumes lower capability owners
and contains only process composition.

### Private compiler-owned command contracts

The executable should define package-private types resembling:

```go
type materialKind uint8

const (
    materialKindSigning materialKind = iota + 1
    materialKindSecret
)

type commandRequest struct {
    Kind        materialKind
    Destination core.AbsoluteFilePath
    SecretBytes core.ByteCount
}

type commandOutcome struct {
    FileActivated bool
    PublicKey     core.Ed25519PublicKeyHex
    Exit          exitCode
}
```

The exact fields require specification review; the important point is that raw
argv closes once into validated typed state. If the library retires
`GenerateCustodySeed`, the command must also retire `custody` unless a distinct
typed derivation-root contract is admitted.

Repeated flags, irrelevant fields, and positionals reject. `Validate()` runs
before entropy or filesystem effects.

### Testable imperative shell

Adopt Kernel's local-`FlagSet`/injected-I/O shape without importing Kernel or
copying its product constants:

```text
main -> construct real dependencies -> run(input) -> typed exit -> os.Exit
```

Only `main` observes process globals. `run` accepts argv, stdout, stderr,
generation operations, persistence capability, and root context in typed
fields. Output errors become observable.

Real-process tests remain mandatory because direct `run` tests cannot prove
actual `os.Args`, flag-package usage, process exit, file descriptors, or binary
wiring.

### Private material stays mutable and bounded

Keep canonical private material in `[]byte` until Filestore finishes. Never
convert it to string, place it in an error, or retain it in a long-lived
request. Clear every command-owned mutable buffer after its final use.

Success and failure tests should use the repository redaction helper over:

- canonical text;
- raw bytes where printable;
- hex and base64 variants;
- generic formatted values; and
- both terminal streams.

### Filestore owns rollback identity

The command may choose create-only policy, but Filestore must own the exact
activated-object rollback/recovery capability. A generic `RemoveFile(root,
path)` call after activation is insufficient.

The command should exhaustively switch on typed write/rollback outcomes:

- no activation;
- activation indeterminate;
- activation requiring directory sync;
- durable activation;
- temporary retained/indeterminate/removal-sync-required/removed;
- rollback completed;
- rollback ambiguous; and
- capability close failure.

Every state maps to a truthful message and exit. No state is collapsed into
`file absent` without identity-backed proof.

### Platform truth

The spec must separately define `private file` for:

- Linux;
- Darwin; and
- Windows.

Unix mode must be verified/corrected through the owned descriptor despite
umask. Windows needs an owner-only ACL contract or an explicit unsupported
result; compiling is not security proof.

Native hostile tests must cover symlink/parent replacement, umask, permissions,
concurrent target replacement, process interruption, and post-activation
recovery on each supported platform.

### Core placement

Core retains only genuinely shared types and stable errors required across
Primitive packages. Command-only tokens, help text, file mode, exit enum,
request shape, and output grammar stay in the command package.

Product file paths, env names, KMS configuration, secret-manager names,
rotation policy, and product messaging never enter Primitive Core or this
generic command.

## Decision rationale and conditions

### Required hostile proof before admission

If a sponsor retains the command, the corrected suite must prove:

1. a typed parser table with every valid kind and hostile unknown/future,
   duplicate, irrelevant, missing, positional, malformed, and extreme flag
   combination;
2. a parser fuzz target with an oracle that either yields a validated request
   or a typed zero/refusal outcome without generation;
3. generation occurs exactly once only after request validation;
4. lexical path rejection happens before generation;
5. each retained kind writes exact Core-canonical bytes without a newline;
6. no private representation reaches stdout, stderr, an error, or generic
   formatting;
7. every stdout/stderr failure maps to the specified exit and activated-file
   outcome;
8. existing files, directories, symlinks, hard links, devices, FIFOs, and
   symlinked/replaced parents remain untouched;
9. every Filestore fault before write, after write, before activation, after
   activation, during directory sync, during recovery, during rollback, and
   during close preserves typed outcome truth;
10. rollback removes only the identity activated by this command;
11. umask and native protection semantics satisfy the platform spec;
12. cancellation and signal arrival at every persistence transition leave a
    typed, recoverable result;
13. every child process in tests has a timeout backstop;
14. the command-specific architecture ratchet permits only its declared lower
    dependencies and rejects raw crypto/filesystem mutation;
15. the production struct inventory classifies request, outcome, and capability
    carriers;
16. direct `run` tests and real built-process tests agree on stdout, stderr,
    exit, and disk outcomes;
17. race, shuffle, coverage, vet, staticcheck, production complexity, and three
    cross-builds pass; and
18. native Linux, Darwin, and Windows evidence exists for every platform the
    spec claims.

### Recon implications

**Do not admit the July 27 command. Retain its mechanics only while a real
operator workflow is adjudicated.**

The archive made a meaningful improvement over its pinned predecessor:
Keygen-plus-Filestore composition, exact Core closure, real process execution,
create-only preservation, recovery attempts, whole-tree refusal snapshots,
public/private linkage, and sliding private-disclosure checks are all valuable.

The blockers are material. The command reports success after output failure,
creates an uncleared private string, accepts ambiguous argv, classifies invalid
requests inconsistently, deletes by path rather than activated identity,
overclaims `0600`, ignores cancellation and time bounds, incompletely proves
non-disclosure, has a radically underspecified contract, puts private CLI facts
in Core, and has no adopted workflow.

Primitive 2026 should make one explicit decision after independent review:

- **retain and rebuild** only if a named generic offline provisioning workflow
  needs a one-file generator and accepts the custody obligations above; or
- **retire the command** while admitting the corrected Keygen library and
  letting Kernel, Witness, Bug, and Peachfuzz keep their typed product-owned
  creation/persistence workflows.

No compatibility layer, consumer migration, commit, or push is authorized by
this interview.

## Independent review

Fresh independent review of this current report is pending. Earlier drafting or
review statements do not count as independent current-file evidence. This
report is complete as recon; it is not independently approved.
