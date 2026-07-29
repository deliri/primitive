# Garble package interview

Status: `COMPLETE` | Decision: `REDESIGN`

This is the exclusive reconstruction report for the archived library package
`garble`. The archive and all four consumer repositories were read-only. The
only source-tree write made by this interview is this report.

The provisional decision is:

- Primitive 2026 has real consumer evidence for a small product-neutral
  Garble contract: derive one deterministic seed from custody plus one
  canonical release identity, bind that seed to an exact supported Garble
  version, and produce the Garble-owned part of a typed build request.
- The July 27 package must not be copied unchanged. It rejects an eight-byte
  all-zero seed that Garble accepts, lets a compiler-valid build plan omit the
  package's version, exports generic serialization and byte-copy surfaces for a
  long-lived custody secret, projects its typed result back to `[]string`, and
  derives a cryptographic protocol version from the repository calendar year.
- The archive's HKDF separation, exact eight-byte projection, redaction,
  canonical parsing, fixed bounds, receiver non-mutation, and copy-isolation
  proofs are worth retaining.
- Bug contains the strongest later consumer design: it probes the installed
  Garble module and version, checks which Go compiler built it, resolves module
  provenance, pins a newer Garble pseudo-version, and attempts to carry
  `garble.Version`, `garble.CustodySeed`, and `garble.Seed` through typed
  structures. That work is not an operable consumer: Bug's production release
  path still imports the older Primitive Release API, while its copied local
  protocol fork imports a `garble` package absent from Bug's Primitive pin and
  vendor tree.

An independent gap/adjudication pass and an actual consumer cutover remain
before this report can become `COMPLETE`.

## Evidence boundary


| Source | Exact revision and Primitive pin | Garble availability | Working-tree qualification |
| --- | --- | --- | --- |
| Archived Primitive | HEAD `d046f7b675fcb797398d7cdc87b5504f43978056` (`2026-07-27T03:35`, `2026-07-27T03:41-04`, `2026-07-27T03:00`, `Harden capability inventory evidence`) | Final `garble` tree `e7b872bb54934ab70cef69f2b5f724fb22a847e6` | One unrelated pre-existing untracked file, `core/api_http_boundary_hostile_test.go`; no archive source changed during this interview. |
| Kernel | HEAD `fec28ef7c9c0ab7e31bfa72127053f96deefcb59`; committed `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:go.mod:76` pins `0df2954a2d911a5d7d775691d023d569affa2c20` (`2026-07-22T21:25`, `2026-07-22T21:01-04`, `2026-07-22T21:00`); dirty `kernel@working-tree:go.mod:76` pins `e8b7172161a4994efcb7f092113e23c28928da43` (`2026-07-27T00:33`, `2026-07-27T00:47-04`, `2026-07-27T00:00`) | Absent at the committed pin; present at the dirty pin, with package and cited Core files byte-identical to archived HEAD | Materially dirty tree. The dirty module pin is evidence, not committed Kernel behavior. Kernel has no `garble` package import at either source state. |
| Witness | HEAD `b9629af57b7058b68982be5d3b282be440b1e76e`; `witness@b9629af57b7058b68982be5d3b282be440b1e76e:go.mod:17` pins `773add8ba0fc1a9453cc06c8558b8541c1fc8ce9` (`2026-07-22T07:30`, `2026-07-22T07:53-04`, `2026-07-22T07:00`) | Absent | Only untracked `.ledger_pending.md`; tracked source and pin match HEAD. Witness uses the older Primitive `release.GarbleSeed` and release-build contract. |
| Bug | HEAD `39ce96242240d7174d562c90bb255860946595dc`; `bug@39ce96242240d7174d562c90bb255860946595dc:go.mod:9` pins `388e593231a28434f6faae9f0ab9dffcf332dfc3` (`2026-07-20T10:59`, `2026-07-20T10:21-04`, `2026-07-20T10:00`) | Absent from the pin and `vendor/modules.txt` | Only untracked `.ledger_pending.md`; tracked source and pin match HEAD. A copied local `protocol/release` fork imports the unavailable final package, while production still imports the old pinned Primitive Release API. |
| Peachfuzz | HEAD `2b2d080c455edaadf88502c1c253845605a4336a`; `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:go.mod:5` pins `3f74d8fc35b4f0f1ddd65ec0e626ee1e06060d75` (`2026-07-23T17:51`, `2026-07-23T17:17-04`, `2026-07-23T17:00`) | Absent | `.ledger_pending.md` is modified. No active Peachfuzz production or test code uses Garble; the only mentions are preserved underscore-prefixed Primitive extraction evidence. |

The final archive package contains ten files and 1,343 lines:

| File class | Files | Lines | Role |
| --- | ---: | ---: | --- |
| Production Go | 4 | 417 | Values, derivation, argument construction, public operations |
| Go tests | 5 | 764 | Hostile value, derivation, formatting, argument, and architecture proof |
| Specification | 1 | 162 | Draft implementation contract |

The package was created and hardened in three rapid revisions:

1. `cf8fa882ee56bb263220a1466c6deae08e466018`
   (`2026-07-24T15:52`, `2026-07-24T15:26-04`, `2026-07-24T15:00`) moved Garble custody, seed derivation,
   version, and command-prefix ownership out of Core and Release.
2. `d259789e87bcadb829c5ffac72c6c91ccc604098`
   (`2026-07-25T21:01`, `2026-07-25T21:06-04`, `2026-07-25T21:00`) moved the package's constants and diagnostic
   formats into Core.
3. `40ded9c104a99cbc4b0b672cd7392901b468d1eb`
   (`2026-07-26T23:14`, `2026-07-26T23:02-04`, `2026-07-26T23:00`) hardened the comparative contract.

The final package did not change after July 26. Kernel's dirty Primitive pin
contains the exact final implementation, but no reviewed consumer adopted it.

## Capability ownership

The archive claims one narrow responsibility:

> Turn a long-lived generic secret and one release identity into an exact
> deterministic Garble seed and the Garble-owned command prefix.

That boundary is stated at `archive@d046f7b675fcb797398d7cdc87b5504f43978056:garble/SPEC.md:8-13`. The public operation
surface is exactly:

```go
func Derive(DeriveRequest) (Seed, error)
func BuildArguments(BuildRequest) (Arguments, error)
func CurrentVersion() Version
```

See `archive@d046f7b675fcb797398d7cdc87b5504f43978056:garble/public.go:8-21` and the AST export ratchet at
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:garble/architecture_test.go:14-29`.

The production data flow is:

```text
core.SecretMaterial + canonical release identity
                     |
                     v
             garble.Derive
         HKDF-SHA-256, 8 bytes
                     |
                     v
                garble.Seed
                     |
                     v
          garble.BuildArguments
                     |
                     v
       Garble-owned build-prefix intent
```

The archive correctly excludes entropy generation, process execution,
repository inspection, product planning, output paths, linker flags, secret
storage, signing, and source transformation
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:garble/SPEC.md:12-13`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:garble/SPEC.md:158-162`).

The 2026 capability should remain this small. The package should not absorb the
release planner or executor merely because they consume its values.

## Archive evidence

### Archived dependency direction

Production imports only the standard library and Core:

```text
crypto/hkdf + crypto/sha256
encoding/base64 + encoding/json
                 |
                 v
              garble --> core values/errors
```

No Primitive sibling or product package is imported. Tests additionally
import `redactiontest`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:garble/secret_format_hostile_test.go:3-8`).

This direction is broadly correct:

- Core may own stable cross-package Garble error identities and any genuinely
  shared release-identity contract.
- Garble should own Garble-specific derivation and tool semantics.
- A release planner should compose Garble's typed intent with typed Go build
  policy, target, output, and import-path facts.
- An executor should resolve and verify the external binary, lower the final
  validated process request to operating-system argv, execute it, and collect
  bounded evidence.

The archive does not yet complete that structure-to-structure chain. Its
`Arguments.Values` method lowers the contract to `[]string` inside Garble
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:garble/arguments.go:37-41`), and neither `BuildRequest` nor
`Arguments` carries a `Version` (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:garble/arguments.go:10-23`).

### Archived mechanics worth retaining

### HKDF replaces the older ad hoc digest

`Derive` validates custody and identity before effect, copies the complete
64-byte root, clears that copy, derives exactly eight bytes through
HKDF-SHA-256, clears the derived temporary, and constructs a validated `Seed`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:garble/derive.go:17-42`).

This is materially better than the older Release implementation, which
concatenated the domain, a separator, release ID, another separator, and
custody bytes into raw SHA-256 before truncation
(`archive@388e593231a28434f6faae9f0ab9dffcf332dfc3:release/seed_custody.go:10-31`). HKDF makes the derivation
roles explicit.

### Exact Garble seed extent and encoding

The package emits exactly eight bytes as strict unpadded standard base64:

- fixed representation: `[8]byte`;
- fixed text extent: 11 bytes;
- strict raw-standard decoder;
- round-trip canonicality check; and
- no reliance on Garble's accepted padding or extra-byte truncation.

See `archive@d046f7b675fcb797398d7cdc87b5504f43978056:garble/values.go:140-174` and
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:garble/SPEC.md:19-27`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:garble/SPEC.md:81-83`.

Garble v0.16.0's own `seedFlag.Set` accepts unpadded or padded standard base64,
requires at least eight decoded bytes, warns and ignores bytes beyond the first
eight, and accepts any exact eight-byte sequence
([official v0.16.0 source](https://github.com/burrowers/garble/blob/v0.16.0/main.go#L506-L541)).
Primitive's stricter exact-length and canonical-encoding boundary is useful;
its extra all-zero exclusion is not supported by that source.

### Fixed and bounded work

The implementation is fixed-bound work over:

- 64 custody bytes;
- at most 128 identity runes;
- eight output bytes; and
- four Garble prefix elements.

There are no goroutines, files, sockets, processes, accumulated histories, or
unbounded reads (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:garble/SPEC.md:122-127`). O(1) memory is the honest
contract here.

### Typed validation and error identity

Every production struct has a `Validate` witness:

- `CustodySeed`;
- `DerivationIdentity`;
- `Seed`;
- `Version`;
- `DeriveRequest`;
- `BuildRequest`; and
- `Arguments`.

See `archive@d046f7b675fcb797398d7cdc87b5504f43978056:garble/values.go:276-281`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:garble/derive.go:45`, and
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:garble/arguments.go:67-70`.

Stable identities are Core-owned:

- `core.ErrGarbleContract`;
- `core.ErrGarbleDerivation`; and
- `core.ErrGarbleArguments`.

They form an `errors.Is` chain through `core.ErrPrimitiveContract`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/error_identity.go:25-27`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:garble/derive_hostile_test.go:53-74`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:garble/arguments_hostile_test.go:42-49`).

### Canonical closure and receiver non-mutation

The value tests attack:

- custody extent and JSON form;
- seed lengths zero through sixteen;
- padding, whitespace, `random`, malformed base64, and invalid UTF-8;
- identity rune/byte width at and beyond the ceiling;
- invalid and oversized JSON; and
- receiver preservation after rejected unmarshalling.

See `archive@d046f7b675fcb797398d7cdc87b5504f43978056:garble/value_hostile_test.go:15-170`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:garble/value_hostile_test.go:172-248`.

The identity width table is particularly useful. It proves that every
one-, two-, three-, and four-byte-rune identity accepted at the 128-rune limit
can decode its own canonical JSON output
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:garble/value_hostile_test.go:172-216`).

### Generic-format redaction

`CustodySeed` and `Seed` implement `fmt.Formatter` and always render
`core.RedactedValueText`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:garble/values.go:19-22`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:garble/values.go:144-148`).

The redaction test searches raw and hexadecimal forms of both custody and
derived seed material across the shared formatter corpus, including zero
values, while preserving the explicit command projection
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:garble/secret_format_hostile_test.go:10-86`).

### Exact prefix and copy isolation

`BuildArguments` constructs exactly:

1. `-seed=<canonical seed>`;
2. `-literals`;
3. `-tiny`; and
4. `build`.

It validates the entire fixed array and returns a copy rather than an alias
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:garble/arguments.go:21-64`). The hostile test mutates the returned
slice and proves the stored value is unchanged
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:garble/arguments_hostile_test.go:12-40`).

## Consumer evidence

### Kernel consumer interview

Kernel does not import Primitive Garble. Its use is an independent local
distribution builder.

Three commands build the Website, Webapp, and Admin binaries, optionally
compress them with UPX, then stream the artifact through SHA-256 and report its
size and elapsed build time
(`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:cmd/lfw/build.go:821-929`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:cmd/lfw/build.go:931-945`).

The Garble invocation is:

```text
garble -seed=random -literals -tiny build ...
```

The executable and flags are package-local strings
(`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:cmd/lfw/build.go:19-52`), and execution uses PATH lookup before
passing a loose argument list
(`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:cmd/lfw/build.go:897-913`).

Kernel's load-test runbook says `v0.16.0 or newer`, installs `@latest`, uses a
random seed, and documents why `-literals`, `-tiny`, `-trimpath`, linker
stripping, and an overlay are applied
(`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:_docs/offgrid_loadtest_garbled_binary.md:27-42`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:_docs/offgrid_loadtest_garbled_binary.md:79-103`).

### Kernel gems

- The actual product use is clear: customer-distributed hardened binaries, not
  every ordinary production build.
- Binary hashing is streaming rather than loading the artifact into memory.
- Product target selection, environment overlay, linker identity, output
  location, UPX, and reporting remain outside the Garble-specific boundary.
- The comments honestly identify a random seed as non-reproducible.

### Kernel defects and boundary lessons

- `random` defeats exact rebuild and must not enter a reproducible release
  contract.
- `v0.16.0 or newer` and `@latest` are unbounded compatibility policies.
- The chosen binary is neither an absolute pre-opened capability nor probed
  for module, version, compiler, or post-execution identity.
- The executable name, flags, and their ordering are copied literals.
- Kernel's dirty Primitive pin includes the final package, but no call site
  uses it. Pin presence is not adoption.

Primitive Garble should preserve Kernel's exact tool-specific intent while a
separate planner/executor owns product and process policy.

### Witness consumer interview

Witness is the strongest working consumer of the older Primitive
Release-owned Garble contract.

It:

- parses a required seed from a compiler-owned environment key;
- rejects `random`;
- constructs a typed per-platform build request containing commit, output,
  import path, build policy, and platform;
- obtains the complete argv from Primitive Release;
- invokes one fixed Garble command per local tool or release target; and
- hashes artifacts through streaming I/O.

See `witness@b9629af57b7058b68982be5d3b282be440b1e76e:cmd/build-tools/build.go:87-136`,
`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/core/release_build.go:92-150`, and
`witness@b9629af57b7058b68982be5d3b282be440b1e76e:cmd/witness-release/main.go:93-138`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:cmd/witness-release/main.go:165-228`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:cmd/witness-release/main.go:557-567`.

Its hostile build table rejects a random seed, zero commit, blank or escaping
output, blank or non-command import path, and unsupported platform before
constructing exact argv
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/core/release_build_test.go:13-96`).

### Witness gems

- Reproducibility is a preflight invariant: exact commit, clean tree, commit
  equality, and non-random canonical seed are decided together
  (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/core/release_preflight.go:13-90`).
- Custody evidence retains only a typed secret-store reference; its struct has
  no seed field, so persisted release custody cannot accidentally serialize the
  secret bytes
  (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/core/release_custody.go:14-31`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/core/release_custody.go:51-80`).
- Platform, output, command, linker policy, and seed are assembled as typed
  structures before the executor sees argv.
- The release manifest records both Go and Garble toolchain versions
  (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/core/release_manifest.go:85-110`).
- Local Witness-owned commands use Garble; remote open-source tools use ordinary
  Go build. That product decision remains downstream
  (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:cmd/build-tools/build.go:87-100`).

### Witness defects and boundary lessons

- Garble and Go versions are accepted from environment strings rather than
  resolved from the executed binaries
  (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:cmd/witness-release/main.go:425-429`).
- `BuildToolchain.Validate` proves only nonblank single-line strings, not the
  package-supported Garble version or its compatibility with Go
  (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/core/release_manifest.go:85-99`).
- The seed travels in environment variables and the in-memory plan as a
  string. The retained reference design is stronger than this ingress.
- Witness's Primitive pin predates the final `garble` package. Its good
  planner and custody mechanics are consumer requirements, not proof that the
  archived API integrates.

The 2026 package should keep Witness's non-random preflight and reference-only
retention model. Tool discovery, compiler compatibility, and execution belong
in the composition/execution boundary.

### Bug consumer interview

Bug contains two different generations of release work.

### Working production path

Production CLI and `internal/release` still import Primitive
`core` and `release` from pin `388e593...`
(`bug@39ce96242240d7174d562c90bb255860946595dc:cli/release.go:16-20`,
`bug@39ce96242240d7174d562c90bb255860946595dc:internal/release/command.go:7-10`).

That path:

- fetches release custody material;
- derives a seed through old `release.DeriveGarbleSeed`;
- builds a typed release plan;
- rejects `random`;
- probes the current Go and Garble toolchain;
- verifies that the Garble binary was built with the same Go version;
- downloads the exact Garble module and records its `go.sum` provenance;
- pins `v0.16.1-0.20260621195108-ffa2daf72f03`;
- builds each planned target;
- inspects and hashes each artifact; and
- runs gates before signing or deployment.

See `bug@39ce96242240d7174d562c90bb255860946595dc:cli/release.go:182-223`,
`bug@39ce96242240d7174d562c90bb255860946595dc:internal/release/facts.go:196-265`,
`bug@39ce96242240d7174d562c90bb255860946595dc:internal/release/plan.go:21-47`,
`bug@39ce96242240d7174d562c90bb255860946595dc:internal/release/pipeline.go:296-326`, and
`bug@39ce96242240d7174d562c90bb255860946595dc:internal/core/release_command_contracts.go:3-22`.

### Copied final-API protocol fork

Bug also contains `protocol/release`, a copied candidate protocol that imports
the final Primitive `garble` package. It improves the intended structures:

- release data carries `garble.Version` and `garble.CustodySeed` and rejects a
  version unequal to `garble.CurrentVersion()`
  (`bug@39ce96242240d7174d562c90bb255860946595dc:protocol/release/release_data.go:60-87`);
- release plans and build requests carry `garble.Seed`
  (`bug@39ce96242240d7174d562c90bb255860946595dc:protocol/release/plan.go:18-25`, `bug@39ce96242240d7174d562c90bb255860946595dc:protocol/release/plan.go:74-83`,
  `bug@39ce96242240d7174d562c90bb255860946595dc:protocol/release/build_plan.go:364-388`); and
- the planner obtains the Garble-owned prefix from `garble.BuildArguments`
  before appending typed Go policy, output, and import-path arguments
  (`bug@39ce96242240d7174d562c90bb255860946595dc:protocol/release/build_plan.go:408-420`).

This fork is not an active production dependency. Bug's pinned Primitive
revision and vendor manifest do not contain `exchange`, `garble`, or
`temporal`. A read-only `go test ./protocol/release` on 2026-07-27 failed at
setup with all three missing packages, including:

```text
cannot find module providing package
github.com/deliri/primitive/v2026/garble:
import lookup disabled by -mod=vendor
```

### Bug gems

- Probe the exact installed Garble module and version rather than trusting an
  operator string.
- Verify the Go compiler embedded in the Garble binary matches the selected Go
  toolchain.
- Bind tool module, version, and module checksum into provenance.
- Pin a precise pseudo-version when a tagged release is not the actual reviewed
  tool.
- Carry custody, seed, and version as their owning types through release data
  and build plans.
- Keep Garble's four-element intent separate from Go build policy, product
  target, output path, execution, inspection, signing, and deployment.

### Bug defects and boundary lessons

- The repository is in a partial migration: the richer local protocol is not
  the protocol used by production.
- The archived package accepts only `v0.16.0`, while Bug pins
  `v0.16.1-0.20260621195108-ffa2daf72f03`. `CurrentVersion` therefore cannot
  satisfy the strongest consumer without an explicit reviewed version choice.
- The local protocol still calls `Arguments.Values()` and appends raw strings.
  It demonstrates the desired ownership split but not the required
  structure-to-structure execution boundary.
- Bug's older CLI material type and derivation still use
  `core.GarbleCustodySeed` and `release.GarbleSeed`
  (`bug@39ce96242240d7174d562c90bb255860946595dc:cli/release.go:102-105`, `bug@39ce96242240d7174d562c90bb255860946595dc:cli/release.go:182-203`), proving the clean cutover is
  unfinished.
- `cloudbuild.release.yaml` installs a substitution-selected tool version while
  claiming the compiler-owned response version will be enforced
  (`bug@39ce96242240d7174d562c90bb255860946595dc:cloudbuild.release.yaml:14-36`). The current production path drops
  the version from its local `releaseMaterial` structure, so that claim must be
  re-proved after migration.

Bug is the best source of new 2026 capabilities, but it is not evidence that
the archived package is already integrated or green.

### Peachfuzz consumer interview

Peachfuzz has no active Garble build, custody, derivation, version, or execution
use at HEAD.

Its only Garble mentions are in `protocol/_foundation_source/core`, an
underscore-prefixed preservation tree excluded from the Go package graph, and
in the extraction record. That record explicitly says copied code is
preservation rather than an approved final API and must be deleted when not
actually needed
(`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:protocol/FOUNDATION_EXTRACTION.md:9-19`, `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:protocol/FOUNDATION_EXTRACTION.md:21-35`).

### Peachfuzz gem

Absence is the requirement: do not make Peachfuzz depend on Garble merely
because Primitive offers the capability. Preserve no copied Garble constants,
errors, or release policy in Peachfuzz.

## Strong mechanics and proof

### Primitive dependents and integration state

The archived Primitive has **zero implemented Go dependents** of `garble`.
Repository-wide production and test import search finds no
`github.com/deliri/primitive/v2026/garble` outside the package itself.

The planned `unleash` package is only a specification. It says Unleash will
consume:

- the exact Garble version;
- deterministic seed derivation;
- the Garble-owned prefix;
- an absolute canonical Garble executable;
- bounded version probes;
- executable identity checks before and after execution; and
- custody/derived-seed containment.

See `archive@d046f7b675fcb797398d7cdc87b5504f43978056:unleash/SPEC.md:24-49`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:unleash/SPEC.md:270-308`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:unleash/SPEC.md:388-396`. There is no Unleash Go
implementation in the archive.

The Release package does not carry `garble.Version`. It carries generic
`core.ToolVersion` in both its request and wire types
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:release/manifest.go:12-28`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:release/json_values.go:36-53`).

This directly contradicts the completed ledger's claim that Release data
directly carries `garble.Version`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:.ledger_completed.md:1010-1012`). The archive's own freeze audit is
more accurate: it leaves Garble `Open` because current local planners still
need indexing
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:_docs/governance/foundation_capability_freeze_audit.md:532-536`).

The honest dependency state is therefore:

```text
garble implementation: no implemented Primitive consumer
garble contract: planned Unleash specification
garble migration: broken Bug protocol fork
older release contract: working Witness and Bug production
local raw invocation: Kernel
no requirement: Peachfuzz
```

## Defects and blockers

### Verified archive defects

### 1. All-zero eight-byte seeds are rejected without a Garble rule

`NewSeed` and `Seed.Validate` reject all-zero material
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:garble/values.go:150-152`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:garble/values.go:176-180`). The specification and tests make
that invented restriction normative
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:garble/SPEC.md:81-83`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:garble/value_hostile_test.go:99-112`).

Garble v0.16.0 checks only decoded length, not byte content
([official source](https://github.com/burrowers/garble/blob/v0.16.0/main.go#L517-L541)).
The older Primitive Release parser also accepted every canonical eight-byte
value (`archive@388e593231a28434f6faae9f0ab9dffcf332dfc3:release/build_plan.go:74-82`).

HKDF can produce the all-zero eight-byte result. The probability is negligible,
but it remains a valid deterministic output. Rejecting it creates an
undocumented partial derivation function and a clean-cut regression from the
older contract. Primitive 2026 must accept it unless a real external
restriction is located and typed.

### 2. Version is not bound to arguments or execution

`BuildRequest` contains only `Seed`; `Arguments` contains only the four strings
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:garble/arguments.go:10-23`). `CurrentVersion` is a separate function
that callers may ignore (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:garble/public.go:18-21`).

Consequently, compiler-valid arguments can be executed by any Garble binary.
The package's exact v0.16.0 behavior claim is an informal convention, not a
compiler-enforced contract.

Bug proves the missing requirement: the chosen tool version, the Go version
that built it, and module provenance all affect admissibility. The final typed
build intent must carry or be structurally paired with the exact supported
version before execution.

### 3. The typed result is lowered to a loose string slice too early

`Arguments.Values` returns `[]string`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:garble/arguments.go:37-41`). Bug and Witness immediately append other
strings and pass the result to `exec.CommandContext`.

That creates an informal protocol seam exactly where Garble-owned flags, Go
flags, paths, linker policy, and product input meet. Copy isolation prevents
mutation of the archived value, but it does not make the resulting command
compiler-owned.

Primitive 2026 should preserve a typed Garble intent until a validated process
request at the execution boundary performs the sole lowering to argv.

### 4. Custody exposes unnecessary generic release surfaces

`CustodySeed` exports:

- `Bytes`;
- `MarshalText`;
- `MarshalJSON`;
- `UnmarshalJSON`; and
- a string parser.

See `archive@d046f7b675fcb797398d7cdc87b5504f43978056:garble/values.go:32-83`.

The specification says the package does not own secret storage, yet generic
JSON and text encoders deliberately release the complete long-lived root
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:garble/SPEC.md:53-58`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:garble/SPEC.md:160-162`). Formatter redaction does not protect
`json.Marshal`, generic text marshaling, or a caller invoking `Bytes`.

Witness supplies the stronger pattern: persistence carries only a typed secret
reference and structurally cannot carry seed bytes. The 2026 package should
expose only the minimum custody ingress needed by a reviewed composition root
and the internal derivation path. Generic secret output should not be admitted
without a located owning boundary.

### 5. Derivation identity borrows a filesystem rule

`DerivationIdentity` validates through `core.ValidateFileNameToken`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:garble/values.go:89-107`). That rule forbids slash, backslash, dot,
and dot-dot because they are filesystem-dangerous
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/path_contracts.go:8-15`).

HKDF `info` is not a filename. The archive has coupled cryptographic identity
to an unrelated path convention, while still accepting broad Unicode without
declaring a normalization rule.

Consumers actually derive from a canonical release identity. The shared
release-identity contract should be compiler-owned in Core if both Garble and
Release need it; Garble should not accept an arbitrary string merely because it
resembles a filename.

### 6. The derivation protocol is implicitly tied to the calendar

`core.GarbleDerivationDomain` is:

```text
foundation-garble-seed-<ContractYear>
```

(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/garble_constants.go:8-10`).

Changing Primitive from 2026 to 2026 therefore changes every derived seed for
the same custody and release identity, even if the derivation algorithm and
Garble contract do not change.

The domain needs its own explicit protocol generation or typed derivation
version. A repository calendar constant is not a substitute for a declared
cryptographic protocol revision.

### 7. Package-private implementation facts are over-centralized in Core

All Garble flags, counts, JSON ceilings, derivation-domain text, seed sizes,
version token, and package diagnostic formats live in Core
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/garble_constants.go:3-18`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/error_constants.go:46-51`).

At archived HEAD no other implemented package consumes them. Stable error
identity belongs in Core under repository policy. Garble-private counts,
diagnostic wording, and algorithm details do not become shared contracts merely
by being constants.

The 2026 cut should retain only facts genuinely shared across package
boundaries in Core and keep Garble's private implementation constants with
Garble. Consumers should receive owning types, not copy or import internal flag
spellings.

### 8. Tool compatibility is only a hard-coded token

`Version` accepts exactly `v0.16.0`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:garble/values.go:230-274`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/garble_constants.go:16-17`).

Garble v0.16.0 itself supports Go 1.26.x but rejects Go 1.27 and also rejects a
Garble binary built with an older Go compiler than the selected toolchain
([official v0.16.0 source](https://github.com/burrowers/garble/blob/v0.16.0/main.go#L547-L579)).

The archive types neither relation. Bug pins a later pseudo-version and proves
compiler matching, so the v0.16.0-only value is already too narrow for the
strongest consumer. A 2026 admission needs one reviewed exact tool version plus
typed compatibility/provenance facts at the execution boundary.

### 9. The ledger overstates integration

The completed ledger claims direct `garble.Version` use by Release, but source
uses `core.ToolVersion`. No package imports Garble, and Unleash has only a
specification.

This is not a cosmetic documentation error. It caused a non-integrated package
to look consumer-proven. The new project must require compiler-visible
dependents and green consumer gates before marking integration complete.

### Verified proof gaps

### Fixed vector is not independently sourced

The test names its case `published fixed vector`, but the only located vector
is a local test constant using a product-specific Witness identity
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:garble/derive_hostile_test.go:11-14`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:garble/derive_hostile_test.go:16-50`). No publication,
cross-implementation oracle, or independent derivation is cited.

The vector is useful as a regression fixture, but it does not independently
prove the RFC 5869 construction. The product token also contradicts the
package's product-neutral boundary.

### Claimed avalanche evidence is absent

The specification requires avalanche supporting evidence
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:garble/SPEC.md:145-146`). The corresponding test only proves that two
custody fixtures contain different bytes
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:garble/derive_hostile_test.go:145-160`). It does not measure or even
compare output-bit changes.

Either add an honest statistical supporting test with a bounded deterministic
oracle or remove the avalanche claim. Determinism and separation are the
contract; avalanche is not a substitute for them.

### No real external-tool conformance test

Tests prove local argv ordering, not that the pinned Garble binary accepts the
plan and produces the intended deterministic result. There is no probe of:

- the installed module path and version;
- the compiler embedded in the Garble binary;
- supported Go range;
- execution with the exact seed/prefix; or
- repeat-build artifact equivalence under controlled inputs.

Bug contains most of the missing fact-resolution design. Unleash specifies the
remaining pre/post executable identity and bounded-output checks. Neither is
implemented in archived Primitive.

### Architecture capability ratchet is string scanning

The forbidden dependency/capability test reads production source and searches
for fragments such as `os/exec`, `net/http`, and `os.Open`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:garble/architecture_test.go:31-62`).

It can be bypassed by import aliasing, alternate APIs, split construction, or
capabilities absent from the list; comments and harmless strings can also
trigger it. The export and struct inventories use ASTs, but capability proof
does not. The 2026 ratchet should inspect imports and declarations
structurally.

### Semantic fuzz coverage is narrow

Only Seed JSON is fuzzed
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:garble/value_hostile_test.go:250-275`). Custody, identity, version,
derive requests, and the version/argument binding have no semantic fuzz oracle.
The Seed fuzz also returns immediately for every rejected input, so it does not
prove stable error identity or receiver preservation on hostile rejection.

### Several hostile tables have numeric or implicit case identities

Custody-size and argument-slot tests name subtests with raw integers
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:garble/value_hostile_test.go:15-42`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:garble/arguments_hostile_test.go:51-68`), while seed sizes and malformed
encodings are loops without semantic subtests
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:garble/value_hostile_test.go:83-113`).

This falls below the testing protocol's requirement that the failing case name
explain the broken property. The actual boundary coverage is valuable; its
case identities need repair.

### No consumer compile proof

`go test ./garble` is green at archived HEAD, but:

- no archived Primitive package imports it;
- Kernel's only pin containing it is dirty and unused;
- Witness and Peachfuzz pins predate it; and
- Bug's candidate fork cannot compile against its pin/vendor tree.

Package-local green is not sufficient admission evidence for a shared release
protocol.

## Primitive 2026 ownership and DAG

### Admit

Admit one package named `garble` if it owns only:

1. a proof-carrying custody input over the Core-owned generic secret material;
2. one compiler-owned canonical release-identity projection;
3. deterministic HKDF-SHA-256 derivation under an explicit Garble derivation
   protocol version;
4. an exact eight-byte seed with strict unpadded standard-base64 projection,
   including the valid all-zero value;
5. one reviewed exact Garble tool version; and
6. the Garble-specific portion of a typed build intent.

Keep entropy in Keygen, release/product planning outside Garble, and process
execution in the owning executor/composition package.

### Retain from the archive

- HKDF-SHA-256 over the complete 64-byte custody root;
- fixed eight-byte output;
- strict canonical encoding;
- validation before derivation and output;
- clearing of package-owned mutable secret copies;
- `fmt` redaction;
- fixed bounds and O(1) memory;
- immutable values and copy isolation;
- typed Core error identities;
- receiver non-mutation;
- rune/byte JSON closure proof; and
- AST public-surface and struct-inventory ratchets.

### Add from consumers

- Witness's rejection of random release seeds;
- Witness's reference-only persisted custody;
- Witness's typed platform/output/import/build-policy request;
- Kernel's streaming artifact hash;
- Bug's exact module/version probe;
- Bug's Garble-compiler versus selected-Go compatibility check;
- Bug's module checksum provenance;
- Bug's typed custody/version/seed projections; and
- Unleash's specified absolute executable, bounded output, isolated build,
  allowlisted environment, and pre/post executable identity checks.

### Reject or redesign

- all-zero seed rejection;
- `ContractYear` as the derivation protocol version;
- file-name validation as the release-identity contract;
- generic custody JSON/text output and public raw bytes without a located
  owner;
- a `BuildRequest` that omits tool version;
- `Arguments.Values() []string` as a package-crossing protocol;
- package-private Garble facts in Core;
- string-scanning capability ratchets;
- environment-only toolchain claims;
- `latest` or `v0.16.0 or newer` version policy; and
- completed-ledger claims without compiler-visible dependents.

## Decision rationale and conditions

### Required admission proof

Before implementation is presented for user review:

1. choose and cite the exact reviewed Garble tool revision for Primitive 2026;
2. type its Go compatibility and provenance boundary;
3. define the canonical Core-owned release identity consumed by both Release
   and Garble;
4. define an explicit derivation protocol generation independent of the
   calendar;
5. decide the minimum custody ingress and remove unowned secret-output
   surfaces;
6. keep Garble intent typed until the executor's final validated argv lowering;
7. add an independently computed RFC 5869 vector and honest separation tests;
8. add a real pinned-tool conformance test with bounded output;
9. migrate one real consumer with no shim, alias, or copied constants;
10. run the package and consumer gates against the same Primitive revision;
    and
11. correct the ledger only after those compiler-visible dependents are green.

### Current verification status

- `go test ./garble` at archived Primitive HEAD: `PASS`.
- Archived package direct dependents: `0`.
- `go test ./protocol/release` at Bug HEAD: `FAIL` at setup because the vendored
  Primitive pin lacks `exchange`, `garble`, and `temporal`.
- Kernel: local raw Garble execution; no Primitive Garble import.
- Witness: working older Release-owned contract; no final Garble import.
- Peachfuzz: no active Garble requirement.
- Primitive Unleash: specification only.

The package-local implementation is coherent, bounded, and well tested, but
the capability is not yet consumer-proven and the defects above remain
admission-blocking.

## Independent review

Fresh independent review of this current report is pending. Earlier drafting or
review statements do not count as independent current-file evidence. This
report is complete as recon; it is not independently approved.
