# `cmd/capabilityinventory` command interview

Status: `COMPLETE` | Decision: `RETIRE`

This is the exclusive reconstruction report for the archived command package
`cmd/capabilityinventory`. It examines the Primitive-only command that read a
five-repository manifest, scanned extracted Go source trees for selected raw
effects and Primitive imports, and wrote the checked-in capability-freeze TSV.

The archive and every downstream repository were read-only. The only
source-tree write made by this interview is this report.

The provisional result is:

- the archive contains unusually strong mechanics for a short-lived governance
  scanner: bounded strict manifest decoding, closed enums, domain-separated
  scalar types, AST matching instead of grep, represented exclusions and
  failures, deterministic output ordering, hostile tables, a parser fuzz
  target, layer triads, and a structural data-flow inventory;
- the exact historical five-archive workflow was independently reproduced
  during this interview and generated the checked-in 103,487-byte artifact
  byte-for-byte;
- the command itself does not prove the Git identities it prints, materialize
  the asserted revisions, bind parsed bytes to stable file identities, bound
  source or aggregate memory, verify its output protocol, or test the real
  executable boundary;
- its closed repository domain is explicitly Kernel, Off Grid Software,
  Witness, Bug, and Peachfuzz, so it is campaign policy rather than a
  product-neutral Primitive runtime capability; and
- no consumer pin contains this package, no inspected consumer invokes it, and
  its only located workflow is the already-frozen Primitive 2026 capability
  audit.

The 2026 direction should therefore be a clean retirement of the archived
production command. If continuing cross-repository inventory is still required,
the useful mechanics should be rebuilt as a private, bounded governance tool
whose provenance and evidence chain are compiler-owned end to end. Nothing here
supports migrating a consumer to this executable or publishing it as a generic
Primitive command.

## Evidence boundary


### Source boundary and exact revisions

### Archived Primitive

The immutable archive inspected here is:

- repository: `/Users/d/code/foundation_back_up_july_27th_2026`;
- HEAD:
  `d046f7b675fcb797398d7cdc87b5504f43978056`;
- HEAD date:
  `2026-07-27T03:35`, `2026-07-27T03:41-04`, `2026-07-27T03:00`;
- HEAD subject:
  `Harden capability inventory evidence`;
- final `cmd/capabilityinventory` tree:
  `0a832f91e768577a7b95c14712dddb7d003b3e1d`; and
- working-tree qualification: one unrelated pre-existing untracked file,
  `core/api_http_boundary_hostile_test.go`.

No archive file was changed during this interview.

The command existed for only two commits:

1. `fbef7492eef4c13919ec0b0a1dc5674154041071`
   (`2026-07-27T02:46`, `2026-07-27T02:31-04`, `2026-07-27T02:00`,
   `Add build-visible capability inventory command`) introduced the command,
   freeze report, and first generated artifact. Its command tree was
   `a41ca4cc8e0752fd0cb198b940e0305b65121524`.
2. `d046f7b675fcb797398d7cdc87b5504f43978056`
   (`2026-07-27T03:35`, `2026-07-27T03:41-04`, `2026-07-27T03:00`,
   `Harden capability inventory evidence`) added the data-flow inventory and
   layer triads, strengthened deterministic ordering and output proof, and
   regenerated the artifact.

The second commit changed the command by 1,520 insertions and 371 deletions
across eight files. This was a concentrated governance campaign, not an API
that evolved through consumer adoption.

### Exact package contents

The final archived package contains eleven files and 4,526 lines:

| File | Lines | Role |
| --- | ---: | --- |
| `data_flow_inventory_test.go` | 333 | Structural inventory of every named production carrier |
| `inventory_hostile_test.go` | 312 | Strict manifest tables, scalar mutation helpers, and manifest fuzzing |
| `layer_triads_test.go` | 535 | Producer, writer, and execute positive/negative/neutral triads |
| `main.go` | 66 | stdin to scan to stdout orchestration and process exit |
| `match.go` | 625 | Capability catalog and AST effect matching |
| `matcher_hostile_test.go` | 198 | Direct matcher, near-miss, catalog, and category tests |
| `model.go` | 955 | Closed manifest, scalar, inventory, violation, exclusion, and error types |
| `output.go` | 292 | Deterministic quoted-TSV projection |
| `scalar_hostile_test.go` | 552 | Scalar domains, enum totals, operation shape, and ordering |
| `scan.go` | 303 | Directory walk, exclusions, parse, imports, violations, and effect aggregation |
| `scanner_output_hostile_test.go` | 355 | Walker/parser failures, generated sources, exclusions, and writer faults |

There is no `cmd/capabilityinventory/SPEC.md`.

That absence is material under the 2026 rebuild contract. The only ownership
description is prose in the freeze audit and a two-line package comment
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/capabilityinventory/main.go:1-3`). It does not define a durable
command specification, resource ceilings, cancellation, platform semantics,
CLI behavior, output grammar, provenance boundary, or required hostile proof.

### Consumer revisions and Primitive pins

| Repository | Current committed HEAD | Primitive pin | Package at the pin | Working-tree qualification |
| --- | --- | --- | --- | --- |
| Kernel | `fec28ef7c9c0ab7e31bfa72127053f96deefcb59` | committed `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:go.mod:76` pins `0df2954a2d911a5d7d775691d023d569affa2c20`; dirty `kernel@working-tree:go.mod:76` pins `e8b7172161a4994efcb7f092113e23c28928da43` | Absent at both revisions | Materially dirty, including the Primitive repin and unrelated product work |
| Witness | `b9629af57b7058b68982be5d3b282be440b1e76e` | `witness@b9629af57b7058b68982be5d3b282be440b1e76e:go.mod:17` pins `773add8ba0fc1a9453cc06c8558b8541c1fc8ce9` | Absent | Only untracked `.ledger_pending.md` |
| Bug | `39ce96242240d7174d562c90bb255860946595dc` | `bug@39ce96242240d7174d562c90bb255860946595dc:go.mod:9` pins `388e593231a28434f6faae9f0ab9dffcf332dfc3` | Absent | Only untracked `.ledger_pending.md` |
| Peachfuzz | `2b2d080c455edaadf88502c1c253845605a4336a` | `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:go.mod:5` pins `3f74d8fc35b4f0f1ddd65ec0e626ee1e06060d75` | Absent | `.ledger_pending.md` is modified |

Exact Git tree queries, not source-search counts, establish the absence:
`cmd/capabilityinventory` does not exist at any of those five Primitive
revisions. All consumer pins predate
`fbef7492eef4c13919ec0b0a1dc5674154041071`, the introducing commit.

## Capability ownership

### Command ownership actually implemented

### Input protocol

The command owns one stdin JSON manifest:

```text
inventoryManifest
|-- Version: inventoryVersion
`-- Repositories: [5]repositorySpec
    |-- Root: core.AbsoluteDirectoryPath
    |-- Head: gitCommitIdentity
    `-- Name: repositoryName
```

The repository array is fixed at
`repositoryNameLimit - 1`, and validation requires its ordinal order to be:

1. Kernel;
2. Off Grid Software;
3. Witness;
4. Bug; and
5. Peachfuzz.

The version has one valid member, `2026-v1`. Repository names, the version, and
Git identities own typed parse/validate/JSON methods
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/capabilityinventory/model.go:18-238`).

`readInventoryManifest`:

- rejects a nil reader;
- limits stdin to `core.StrictJSONMaxBytes + 1`;
- rejects content above the strict JSON ceiling;
- decodes through `core.DecodeStrictJSON`;
- wraps malformed structure with a typed command error; and
- returns only a validated manifest
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/capabilityinventory/main.go:43-65`).

Leading and trailing JSON whitespace are deliberately accepted and tested
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/capabilityinventory/inventory_hostile_test.go:17-55`).

### Scan domain

For each repository root, the command:

- recursively walks the filesystem with `filepath.WalkDir`;
- ignores `_test.go`;
- parses production `.go` files with comments and skipped object resolution;
- excludes sources recognized by Go's `ast.IsGenerated`;
- records Primitive import package/file pairs;
- records selected direct and candidate raw effects;
- emits exclusions for `.git`, `vendor`, `node_modules`, `.cache`, `dist`,
  `testdata`, and exact `protocol/_foundation_source`;
- represents walk, empty repository, path, source kind, read, parse, and dot
  import failures as violations; and
- continues scanning where possible instead of silently shrinking evidence
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/capabilityinventory/scan.go:17-303`).

The scanner deliberately operates at source-AST level, not type-checker level.
It matches:

- direct package selectors such as `time.Now`, `os.WriteFile`,
  `exec.Command`, and `rand.Read`;
- import-only families such as `math/rand`, cloud SDK prefixes, and platform
  ABI packages;
- `exec.Cmd` composite literals and allocations; and
- method-name candidates when the file imports a relevant package
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/capabilityinventory/match.go:277-395`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/capabilityinventory/match.go:421-625`).

The closed catalog contains seventeen categories:

- raw time;
- raw context deadlines;
- raw context cancellation;
- raw filesystem mutation;
- filesystem-handle method candidates;
- raw network client/server construction;
- network method candidates;
- raw network sockets;
- raw signals;
- raw process construction;
- process method candidates;
- raw environment;
- raw runtime-memory mutation;
- raw entropy;
- raw pseudorandom imports;
- cloud SDK imports; and
- raw platform ABI imports
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/capabilityinventory/model.go:240-342`).

### Output and refusal

The command writes a quoted, tab-separated record stream to stdout. Its record
kinds are:

- `TOOL`;
- `REPOSITORY`;
- `FOUNDATION`;
- `CAPABILITY`;
- `USE`;
- `VIOLATIONS`;
- `VIOLATION`;
- `EXCLUSIONS`; and
- `EXCLUSION`
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/capabilityinventory/output.go:10-80`).

It validates the in-memory inventory before output. Repositories are emitted in
manifest order. Primitive imports, capability files, operations, exclusions,
and violations are sorted before emission. Every category is represented even
when its count is zero
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/capabilityinventory/output.go:86-265`).

Each line is assembled with `strconv.AppendQuote`, written in one `Write` call,
and checked for both writer error and short write
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/capabilityinventory/output.go:268-291`).

The execute boundary intentionally writes the complete represented inventory
before deciding whether represented violations make the command fail. If any
repository has a violation, it returns a typed Primitive contract error after
output (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/capabilityinventory/main.go:22-40`).

`main` prints any returned error to stderr and exits `1`; success exits `0`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/capabilityinventory/main.go:15-20`).

### What the command does not own

The executable does not:

- invoke Git;
- verify that a manifest `Head` exists;
- verify that a `Root` contains that `Head`;
- materialize a commit archive;
- reject a dirty source checkout;
- create or atomically replace the checked-in TSV;
- calculate or update the TSV's SHA-256 contract;
- parse or independently verify its own output;
- conduct the semantic package interviews required after AST discovery;
- classify product ownership;
- decide package admission or retirement; or
- migrate any consumer.

Those effects are either described in prose, performed by an operator, or
owned by separate Core tests. They are not part of the command's compiled
boundary.

## Archive evidence

### Located workflows and consumers

### Primitive capability-freeze workflow

The sole located operational use is the 2026 Primitive capability-freeze
audit:

- the audit declares the generated `REPOSITORY` records as the source of truth
  for frozen downstream heads
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:_docs/governance/foundation_capability_freeze_audit.md:33-54`);
- it uses the AST artifact as one of two passes, with semantic interviews as
  the second pass
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:_docs/governance/foundation_capability_freeze_audit.md:60-77`);
- it accurately describes the command as a reproducible discovery floor, not a
  semantic ceiling
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:_docs/governance/foundation_capability_freeze_audit.md:79-114`);
- it specifies a manual five-step process: verify each Git identity, extract
  each commit with `git archive`, construct a manifest, run the command, and
  compare output byte-for-byte
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:_docs/governance/foundation_capability_freeze_audit.md:116-133`);
  and
- the checked-in output is
  `_docs/governance/foundation_capability_effect_inventory.tsv`.

The artifact is:

- 1,971 lines;
- 103,487 bytes;
- SHA-256
  `8c4bdffe65556ff5dcd67e15c20478b60e43c304766c8923e5688474c63912ba`;
  and
- composed of five repository sections with zero represented violations
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:_docs/governance/foundation_capability_effect_inventory.tsv:1-1971`).

Its frozen repository identities are:

| Repository | Artifact HEAD | Production Go files | Primitive-importing files |
| --- | --- | ---: | ---: |
| Kernel | `fec28ef7c9c0ab7e31bfa72127053f96deefcb59` | 708 | 167 |
| Off Grid Software | `69cb846f69e8d5304ebbb11eb5d7c2a4ea7c270f` | 120 | 63 |
| Witness | `b9629af57b7058b68982be5d3b282be440b1e76e` | 507 | 163 |
| Bug | `39ce96242240d7174d562c90bb255860946595dc` | 191 | 99 |
| Peachfuzz | `2b2d080c455edaadf88502c1c253845605a4336a` | 203 | 126 |

The section boundaries and zero-violation records are located at
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:_docs/governance/foundation_capability_effect_inventory.tsv:2`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:_docs/governance/foundation_capability_effect_inventory.tsv:581-603`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:_docs/governance/foundation_capability_effect_inventory.tsv:800-821`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:_docs/governance/foundation_capability_effect_inventory.tsv:1344-1354`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:_docs/governance/foundation_capability_effect_inventory.tsv:1651-1657`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:_docs/governance/foundation_capability_effect_inventory.tsv:1967-1971`.

During this interview, all five exact commits were freshly materialized with
`git archive`; a new typed manifest was supplied to the archived command; and
its stdout compared byte-for-byte with the checked-in artifact. The comparison
passed.

That is strong historical evidence for deterministic happy-path regeneration.
It does not make the manual provenance procedure a compiler-owned production
contract.

### Core governance-document ratchet

Archived Core also knows the artifact as one of five governance documents:

- `GovernanceDocumentPrimitiveCapabilityEffectInventory` is a closed enum
  member;
- its path is
  `_docs/governance/foundation_capability_effect_inventory.tsv`; and
- its expected SHA-256 is the exact digest above
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/governance_contracts.go:8-27`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/governance_contracts.go:56-143`;
  `archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/governance_constants.go:27-45`).

`requireGovernanceDocument` reads the checked-in file, enforces a nonempty
512-KiB ceiling, hashes it, and compares it with the Core-owned expected digest
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/governance_contracts_test.go:80-121`).

This protects the checked-in bytes from silent drift. It does not prove those
bytes came from the asserted repository commits; the command and Core test own
opposite ends of the chain with a manual gap between them.

## Consumer evidence

### Kernel

Kernel's committed and dirty Primitive revisions do not contain the command.
No `cmd/capabilityinventory`, tool token, artifact path, or capability-effect
vocabulary was located in Kernel production, test, or governance files.

Kernel does contain many targeted AST structure tests under `sentinel/` and a
separate `cmd/teststats` AST generator
(`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:cmd/teststats/main.go:1-398`;
`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:cmd/teststats/teststats.md:1-7`). Those are product-owned ratchets and a
product statistics generator, not copies or consumers of the cross-repository
capability inventory.

### Witness

Witness's pinned Primitive revision does not contain the command. No
invocation or artifact consumer was located.

Witness owns doctrine analyzers under `internal/doctrine`, including AST-based
package-capability and streaming checks
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/doctrine/layers.go:1-720`;
`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/doctrine/streaming.go:1-191`). These enforce current Witness
test doctrine; they do not reproduce the archive's five-product effect
inventory.

If a generic ongoing analyzer is required in 2026, overlap with the
Primitive-pinned Witness linter must be resolved before creating another AST
owner. The archive does not establish a reason for two tools to own the same
doctrine fact.

### Bug

Bug's pinned Primitive revision does not contain the command. No invocation or
artifact consumer was located.

Bug uses AST parsing inside its own run/proof and behavioral-binding surfaces
(`bug@39ce96242240d7174d562c90bb255860946595dc:internal/run/behavioral_binding.go:1-430`;
`bug@39ce96242240d7174d562c90bb255860946595dc:internal/run/proof.go:1-700`). Those are product execution/evidence
mechanics, not repository-wide capability discovery.

### Peachfuzz

Peachfuzz's pinned Primitive revision does not contain the command. No
invocation or artifact consumer was located.

Peachfuzz's production AST use is product discovery and test-contract support
(`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/discover/discover.go:1-380`;
`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/testcontract/data_flow_scan.go:1-80`). It does not consume
the archived TSV protocol.

### Workflow conclusion

The command had one legitimate, Primitive-owned use: construct a reproducible
discovery artifact for a bounded 2026 retirement campaign. The exact artifact
is useful historical evidence. No current consumer requires the executable,
and no general operator workflow, installation path, release target, or
recurring gate was located.

Absence from consumers is not by itself retirement evidence. Here retirement
is supported by the combination of:

- explicit product identities and 2026 versioning in the command;
- a single located Primitive campaign;
- no package at any consumer pin;
- no current invocation;
- an already-sealed historical output;
- mandatory semantic interviews that supersede the scanner's discovery rows;
  and
- current 2026 genericity rules that forbid product policy in Primitive
  production packages.

## Strong mechanics and proof

### Archive strengths worth preserving

### Closed manifest shape

The fixed repository array makes omission, duplication, and reordering invalid
states instead of runtime conventions. Repository names and version tokens are
closed enums, and a Git SHA-1 object identity is carried as exactly twenty
bytes rather than a loose string
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/capabilityinventory/model.go:18-238`).

The hostile manifest table attacks:

- empty, null, scalar, array, and truncated inputs;
- trailing values;
- invalid UTF-8;
- unknown and duplicated JSON fields;
- missing, duplicated, and reordered repositories;
- unknown and wrong-case names;
- zero, uppercase, short, long, and non-hex commit identities;
- relative, unclean, duplicate, and empty roots;
- wrong field types;
- over-ceiling input; and
- a nil reader
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/capabilityinventory/inventory_hostile_test.go:57-135`).

Rejected manifests return the zero value and a typed command error with stable
Primitive identity.

### Domain-separated scalars

`SourcePath`, `ImportAlias`, `ImportPath`, `ImportPrefix`, and
`SelectorName` are distinct structs with zero-field domain tags. The archive
uses this shape to prevent implicit cross-conversion even though the runtime
payload of each type is a string
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/capabilityinventory/model.go:344-480`).

The tests prove:

- the types do not share the same Go type;
- each domain value has the intended width;
- source paths are local, clean, slash-canonical, `.go`, control-free UTF-8,
  and bounded;
- aliases and selectors follow Go identifier grammar;
- import paths and prefixes are canonical and bounded; and
- unknown/future enum ordinals fail
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/capabilityinventory/scalar_hostile_test.go:13-552`).

This is a valuable compiler-owned pattern for any replacement.

### AST-aware matching

The scanner resolves explicit and default import aliases before attributing
direct selectors. It does not confuse a similarly named third-party package
with a standard-library import. It recognizes explicit aliases, dot imports,
import-only capabilities, process composite literals, and `new(exec.Cmd)`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/capabilityinventory/scan.go:232-283`;
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/capabilityinventory/match.go:421-585`).

The direct matcher table covers more than thirty distinct effect shapes across
all closed categories. Its near-miss table rejects type-only imports,
read-only filesystem calls, constants, similarly named cloud/exec packages,
methods without a required import, and fields that merely share a method name
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/capabilityinventory/matcher_hostile_test.go:9-104`).

### Honest candidate labeling

The archive does not claim method-name matches are semantically resolved. It
names the categories and rendered operations `method_candidate`, and the audit
explicitly says the AST result is a floor requiring semantic interview
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/capabilityinventory/model.go:563-577`;
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:_docs/governance/foundation_capability_freeze_audit.md:106-114`).

That distinction should survive if the discovery mechanic survives.

### Failure representation

Walk, source kind, read, parse, path, empty repository, and dot-import failures
become output records instead of disappearing from counts. The command then
fails after the represented artifact is written.

The scanner failure table injects:

- a missing runtime;
- walker failure;
- source read failure;
- parse failure;
- nil directory entries;
- symbolic-link source entries;
- an outside-root path; and
- an unattributable dot import
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/capabilityinventory/scanner_output_hostile_test.go:15-94`).

This is materially stronger than a scanner that silently skips unreadable
files and produces a reassuringly small report.

### Explicit exclusion evidence

Every intentionally skipped directory produces a typed exclusion row. Tests
cover all seven excluded directory shapes and prove the copied-Primitive
special case requires its exact path boundary
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/capabilityinventory/scan.go:149-184`;
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/capabilityinventory/scanner_output_hostile_test.go:180-235`).

### Deterministic structural ordering

Output ordering is explicit. In particular, effect operations are sorted by
their structured import, selector, and kind fields, not their display string.
The archive records the collision that motivated this: an import-only
`example.com/a.b` and selector `example.com/a` + `b` render identically
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/capabilityinventory/model.go:600-626`).

The test constructs all operation kinds around that collision and proves a
total structural order
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/capabilityinventory/scalar_hostile_test.go:81-145`).

The successful byte-identical regeneration during this interview provides
end-to-end evidence that happy-path ordering is deterministic for the frozen
five repositories.

### Writer fault handling

The output writer rejects:

- nil destinations;
- wrong field counts;
- writer errors; and
- short writes.

Those failures retain typed `inventoryErrorKindOutput` identity
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/capabilityinventory/output.go:86-103`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/capabilityinventory/output.go:268-291`;
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/capabilityinventory/layer_triads_test.go:151-177`).

### Structural data-flow inventory

The final archive has a narrow AST ratchet that discovers every named
production type and requires it to appear in a closed role inventory. It
classifies protocol types, enums, domain tags, domain values, catalog types,
internal flow, operating seams, output, and error carriers. Role-specific
obligations require parse/validate methods where appropriate, and domain tags
must remain zero-field structs
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/capabilityinventory/data_flow_inventory_test.go:13-185`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/capabilityinventory/data_flow_inventory_test.go:187-333`).

This is a genuine compiler-shape ratchet, not a prose list.

### Testing-protocol proof

### Evidence mapping

| Testing-protocol rule | Archived proof | Status |
| --- | --- | --- |
| `test/compiler-driven` | Fixed arrays, closed enums, domain-separated scalars, typed error kind, `Validate()` boundaries | Strong internally |
| `test/evidence` | Every hostile row checks a concrete effect, rejection, output record, exclusion, or violation | Strong |
| `test/red-green-slice` | Commit history and comments identify deterministic ordering and evidence-shrink failure classes | Strong for the final hardening slice |
| `test/isolation/tempdir` | Real filesystem triads create `t.TempDir()` inside parallel subtests | Conforming |
| `test/parallel/default` | Every top-level test and eligible subtest is parallel; fuzz owns its callback | Conforming |
| `test/sync/no-sleep` | No sleep-based synchronization | Conforming |
| `test/determinism` | Fixed identities, temp roots, structural sorts, exact quoted fields, byte-identical archive regeneration | Strong happy path; failure diagnostics remain platform-derived |
| `test/table-shape` | Large hostile manifest, scalar, matcher, generated-file, failure, and exclusion tables | Strong |
| `test/boundaries` | Exact length, canonicality, zero/unknown enum, malformed JSON, and path boundaries | Strong for modeled scalars |
| `test/layer-triad` | Named producer, writer, and execute `LayerTriad` tests | Present and non-vacuous |
| `test/package-sweep-exit` | Focused gates pass, but specification, provenance, process, resource, and verifier surfaces remain open | Not satisfied |
| `test/errors` | `errors.Is(core.ErrPrimitiveContract)` plus `errors.AsType[capabilityInventoryError]` and exact kind | Strong |
| `test/structural-equality` | No `reflect.DeepEqual`; direct equality and `slices.Equal` are used | Conforming |
| `test/helpers` | Plain Go helpers preserve got/want facts | Conforming |
| `test/fixtures/no-shared-mutable` | Fixtures are local; no shared mutable package globals | Conforming |
| `test/fixtures/non-vacuous` | Layer rows assert exact production-file, effect, violation, and record counts | Strong |
| `test/production-path` | Real filesystem scanning reaches `executeCapabilityInventory`; no built-process test reaches `main` | Partial |
| `test/evidence-path-consistency` | Checked-in artifact has a Core-pinned digest, but no test proves the chain from manifest to exact Git trees to artifact to digest | Incomplete |
| `test/goroutines/owned` | No goroutines are created | Truthfully `NOT_APPLICABLE` |
| `test/structural-invariant` | AST data-flow carrier inventory is narrow and tied to a real unclassified-carrier risk | Strong |
| `test/data-flow-inventory` | Every named production type is discovered and classified | Strong |
| `test/repeat-policy` | Repeated Go tests were exercised; tool and fuzz phases are not internally repeat-amplified | Conforming |
| `test/budget-divergence` | Command owns no run-profile budget | Truthfully `NOT_APPLICABLE`, but command resource ceilings are still missing |
| `test/benchmarks` | No performance claim is made | Truthfully `NOT_APPLICABLE` |
| `test/fuzz-boundary` | Manifest fuzz accepts only validated values, requires typed rejection, and proves canonical round trip | Strong for manifest input |
| `test/ledger-chain` | No ledger is owned | Truthfully `NOT_APPLICABLE` |
| `protocol/typed-boundary` | Manifest is typed; output crosses through a loose `[]string` sequential grammar | Violated at output |
| `test/waivers` | No waiver is needed or present | Conforming |

### Reproduced package gates

The following commands were run against the immutable archive:

| Gate | Result |
| --- | --- |
| `go test ./cmd/capabilityinventory -count=1` | PASS |
| `go test -race ./cmd/capabilityinventory -count=1` | PASS |
| `go test ./cmd/capabilityinventory -shuffle=on -count=10` | PASS |
| `go test ./cmd/capabilityinventory -cover -count=1` | PASS, 81.9% statement coverage |
| `go vet ./cmd/capabilityinventory` | PASS |
| `staticcheck ./cmd/capabilityinventory` | PASS |
| `gocyclo -over 10 cmd/capabilityinventory` | No production function over 10 |
| `fieldalignment ./cmd/capabilityinventory` | PASS |
| `gofmt -d cmd/capabilityinventory` | No diff |
| Linux/amd64 test-binary cross-build | PASS |
| Darwin/arm64 test-binary cross-build | PASS |
| Windows/amd64 test-binary cross-build | PASS |

Whole-directory complexity reports only four test functions above ten:

- `TestCapabilityInventoryExecuteLayerTriad`: 15;
- `TestEffectOperationOrderIsStructuralNotRendered`: 13;
- `TestCapabilityEffectProducerLayerTriad`: 11; and
- `productionNamedTypes`: 11.

Production complexity is clean.

Cross-building proves compile portability only. It is not native Windows,
Linux, or Darwin behavior proof.

### Reproduced artifact gate

The interview additionally:

1. verified each frozen repository commit exists in its source repository;
2. created five fresh empty temporary roots;
3. materialized each exact commit with `git archive`;
4. supplied a fresh manifest to
   `go run ./cmd/capabilityinventory`; and
5. compared stdout with the checked-in TSV.

The comparison was byte-identical and exited successfully. The temporary roots
were then removed.

This is stronger than the package tests, because it drives the real command and
the actual archived inputs. It is still an interview reproduction, not a
checked-in compiler-owned gate.

## Defects and blockers

### B1: no 2026 specification owns the command

There is no package-local `SPEC.md`. The two-line command comment says only
that the tool produces a pinned raw-effect inventory for Primitive's
retirement campaign (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/capabilityinventory/main.go:1-3`).

Missing contracts include:

- whether this is one-shot migration tooling or a supported command;
- accepted arguments and environment;
- stdin and stdout grammar;
- provenance ownership;
- repository selection policy;
- source and aggregate size ceilings;
- cancellation and time bounds;
- platform behavior;
- evidence finalization;
- stable errors and exit classes;
- semantic false-positive/false-negative policy; and
- retirement conditions.

Under `AGENTS.md`, production cannot be admitted before those facts have one
owner. The archive must not be copied first and specified afterward.

### B2: repository identities are assertions, not verified provenance

`repositorySpec.Head` is validated only as nonzero canonical lowercase
forty-character hex (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/capabilityinventory/model.go:140-213`).

The scan request uses `Spec.Root` for `WalkDir`, but no scan path reads,
compares, or otherwise uses `Spec.Head`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/capabilityinventory/scan.go:54-105`).

The head is repeated verbatim into the output summary
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/capabilityinventory/output.go:128-137`).

Consequently, a caller can pair:

- Kernel's asserted commit with Peachfuzz's root;
- any invented nonzero forty-hex identity with an arbitrary source directory;
  or
- a clean asserted HEAD with a dirty working tree.

The output will print the assertion as if it described the scanned bytes.

The tests prove this behavior unintentionally. `testInventoryManifestAt`
constructs synthetic nonzero byte-pattern commit identities, creates ordinary
temporary directories with no Git metadata, and the full execute triad accepts
and prints them
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/capabilityinventory/inventory_hostile_test.go:176-212`;
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/capabilityinventory/layer_triads_test.go:200-288`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/capabilityinventory/layer_triads_test.go:428-455`).

The freeze audit acknowledges the division and delegates identity verification
and archive materialization to a manual procedure
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:_docs/governance/foundation_capability_freeze_audit.md:84-89`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:_docs/governance/foundation_capability_freeze_audit.md:116-128`).

That procedure worked when reproduced. It is nevertheless outside the compiler
and outside the command. A 2026 evidence-producing tool must either:

- own Git commit verification and materialization through a typed, bounded
  process capability; or
- accept a typed materialization attestation that cryptographically binds each
  asserted commit to the exact scanned tree and independently verify it.

Repeating an unverified identity is not provenance.

### B3: output is a loose, implicit sequential protocol

The in-memory model is typed, but the external boundary is:

```go
write(kind inventoryRecordKind, fields []string)
```

Every record is flattened into `[]string`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/capabilityinventory/output.go:268-283`).

The protocol also relies on ordering context:

- `FOUNDATION` has package and file, but no repository;
- `CAPABILITY` has category and count, but no repository;
- `USE` has file and operation, but neither repository nor category;
- `VIOLATION` omits repository even though the in-memory violation carries it;
  and
- `EXCLUSION` omits repository.

A consumer can associate those records only by remembering the preceding
`REPOSITORY` and `CAPABILITY` records
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/capabilityinventory/output.go:112-249`).

The only decoder is test-only and returns
`decodedInventoryRecord{Kind, Fields []string}`. It validates each local field
count but does not reconstruct or validate repository/category nesting
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/capabilityinventory/layer_triads_test.go:16-19`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/capabilityinventory/layer_triads_test.go:381-425`).

This violates the 2026 structure-to-structure and typed-protocol rules. A
replacement must emit self-contained typed records or a typed bounded document,
and it needs a production-grade parser/verifier if the artifact is treated as
evidence.

### B4: the evidence chain is split across three owners

The archived evidence chain is:

```text
prose/manual Git procedure
        |
        v
capabilityinventory stdout
        |
        v
manual checked-in TSV update
        |
        v
Core path + expected SHA-256
        |
        v
Core governance-document test
```

The end digest is real and matches the checked-in bytes. The regenerated bytes
are real and matched during this interview. What is absent is a single
compiler-owned chain proving:

```text
asserted ref
AND materialized tree identity
AND scanner version/catalog
AND typed records
AND durable artifact bytes
AND manifest/digest
```

No checked-in manifest, regeneration script, provenance record, output
verifier, or gate invocation was located. The only procedure is prose
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:_docs/governance/foundation_capability_freeze_audit.md:116-133`).

The Core test proves file presence, size, and hash only
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/governance_contracts_test.go:103-121`). A forged artifact can be
ratcheted by changing its digest without the compiler proving its derivation.

This falls short of `test/evidence-path-consistency`,
`test/production-path`, and the hostile self-consistency layer.

### B5: source and aggregate resource use are unbounded

Only the JSON manifest has a byte ceiling
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/capabilityinventory/main.go:43-55`).

Production source parsing calls `parser.ParseFile` directly on a path with no
source-size limit (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/capabilityinventory/scan.go:33-40`).

The command then retains, for all five repositories:

- every Primitive-importing file;
- every Primitive package/file pair;
- every capability file/category key;
- every matched operation set;
- every violation;
- every exclusion; and
- all repository inventories until output
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/capabilityinventory/model.go:754-878`;
  `archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/capabilityinventory/scan.go:60-76`).

There is no ceiling for:

- bytes per Go file;
- files per repository;
- directory depth;
- imports per file;
- effects per file;
- violations;
- exclusions;
- total retained records; or
- output bytes.

This is O(total scanned world) memory, not a bounded streaming pipeline. A
replacement must own explicit per-file and aggregate limits and stream sorted
partitions or use bounded external spill with clear ownership.

### B6: no cancellation or deadline reaches scanning

The command accepts only `io.Reader` and `io.Writer`; neither execute nor scan
takes a context
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/capabilityinventory/main.go:22-40`;
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/capabilityinventory/scan.go:60-105`).

It can block indefinitely on:

- stdin;
- a stalled or adversarial filesystem walk;
- a special filesystem parse/open;
- a very large source file; or
- blocked stdout.

There is no signal handling, cancellation path, timeout, or bounded shutdown.
No goroutine leaks exist because the command starts no goroutines, but absence
of goroutines is not bounded execution.

### B7: the file-kind check and parsed bytes are not identity-bound

The walker receives a `DirEntry`, rejects entries whose reported type is not a
regular file, and later passes the path string to `parser.ParseFile`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/capabilityinventory/scan.go:115-139`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/capabilityinventory/scan.go:190-205`).

The regular-file check and open are separate path operations. An entry can be
replaced between them. The parser follows the path that exists at open time,
so a race can:

- redirect the scan outside the materialized tree;
- substitute different bytes after classification; or
- make the result depend on concurrent mutation.

The manifest root itself is only a validated absolute path; the scan does not
hold a directory capability or an immutable archive handle.

The symbolic-link test proves only that a `DirEntry` already reported as a
symlink is represented as a violation
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/capabilityinventory/scanner_output_hostile_test.go:15-94`).
It does not attack replacement between walk and parse.

Fresh `git archive` roots reduce this risk operationally but do not make the
command's boundary correct. A retained tool should scan immutable materialized
content or bind open handles and identity facts across inspection.

### B8: AST discovery has acknowledged and untested semantic gaps

The audit correctly says the scanner cannot:

- prove concrete method receivers;
- resolve a default import name that differs from the path base; or
- see product wrappers
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:_docs/governance/foundation_capability_freeze_audit.md:106-114`).

Additional verified consequences follow directly from the matcher:

- a local identifier that shadows an imported alias can be attributed to the
  imported package because `selectorImportPath` consults only the file import
  map (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/capabilityinventory/match.go:473-500`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/capabilityinventory/match.go:523-530`);
- any call named `Write`, `Run`, `Do`, and similar can become a method
  candidate whenever the file imports the required package, regardless of
  receiver type
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/capabilityinventory/match.go:586-625`);
- generated sources are wholly excluded after parsing, even when generated
  production code contains the only raw effect
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/capabilityinventory/scan.go:201-208`);
- build constraints and target platform are not evaluated, so mutually
  exclusive files are aggregated into one source inventory; and
- the `raw_time` category includes pure constructors and parsers such as
  `time.Date`, `time.Parse`, `time.Unix`, and `time.FixedZone`, which do not
  observe a raw clock
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/capabilityinventory/match.go:277-287`).

Candidate labeling makes many false positives honest, but direct-selector
shadowing can still overstate facts and nonstandard default package names can
miss facts. The artifact remains a discovery lead set only. It cannot be a
semantic ownership verdict.

No fuzz target attacks Go source parsing/matching, and the near-miss table does
not cover local shadowing or an unrelated called method with the same name in a
file that imports the relevant package.

### B9: exclusion and repository policy are embedded in production code

The command hardcodes:

- five product repository names
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/capabilityinventory/model.go:72-138`);
- the exact Primitive module import prefix
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/capabilityinventory/match.go:440-453`);
- the campaign's fixed directory exclusions
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/capabilityinventory/scan.go:149-170`); and
- cloud-provider package prefixes
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/capabilityinventory/match.go:378-393`).

Those are legitimate facts for the 2026 campaign, but they are not generic
Primitive runtime contracts. Retaining the executable under production
`cmd/` would make Primitive know its consumers and campaign policy, contrary
to 2026 genericity.

A private governance tool may receive a validated policy catalog from its
campaign owner. A reusable production package must not encode product names.

### B10: the real CLI/process envelope is unproved and ambiguous

All package tests call helpers or `executeCapabilityInventory`; none builds and
executes the actual binary. There is no `TestMain`, subprocess harness, or
process table.

Unproved behavior includes:

- stdin inheritance and closure;
- stdout broken-pipe behavior;
- stderr write failure;
- exact terminal diagnostics;
- exit status;
- signal interruption;
- partial stdout on writer failure; and
- the relationship between represented violations, stdout, stderr, and exit.

The command also never inspects `os.Args`. Arbitrary trailing arguments are
silently accepted because `main` always reads stdin and calls execute
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:cmd/capabilityinventory/main.go:15-20`).

There is no help, version, usage, or refusal contract. A 2026 command must own
one typed invocation grammar and reject unexplained arguments.

### B11: the checked-in artifact is sealed, but regeneration is not a gate

The final archive pins the artifact digest in Core and the digest currently
matches. That is a useful static ratchet.

However:

- no package test materializes the five repository commits;
- no test regenerates the checked-in artifact;
- no source records the manifest used for generation;
- no gate script invokes the command;
- no test proves repeat generation is byte-identical;
- no test proves the frozen refs are still available; and
- no failure-injection test spans Git materialization through durable artifact
  replacement and digest update.

This interview independently reproduced the artifact, but a future reviewer
should not need an ad hoc procedure to recover the evidence chain.

### B12: command-specific governance facts leaked into archived Core

Archived Core owns:

- the capability-effect-inventory document enum;
- its filename;
- its path;
- and its exact SHA-256
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/governance_contracts.go:8-27`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/governance_contracts.go:110-143`;
  `archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/governance_constants.go:27-45`).

Those facts were shared by the command's governance workflow and Core's
ratchet, so the archive centralized them consistently. For 2026, the ownership
question changes: if the command and artifact are retired historical campaign
material, their tokens and digest do not belong in generic production Core.

The replacement owner, if any, should be one private governance-tool package or
its typed evidence schema. Core should own only contracts shared by admitted
Primitive production packages.

## Primitive 2026 ownership and DAG

### Required 2026 shape if an ongoing tool is justified

The default verdict is retirement. If the root review establishes a real
recurring Primitive governance workflow, a replacement must be a clean build,
not a compatibility port.

### Placement

Prefer a private build-visible governance tool, such as an admitted `_tools`
command, over a published production `cmd` package.

The package boundary must say:

- the tool is Primitive governance infrastructure;
- the repository list and capability policy belong to the invoking campaign;
- no consumer imports or executes it;
- its output is evidence, not an admission verdict; and
- retirement occurs when the owning campaign ends unless a recurring gate is
  explicitly adopted.

Overlap with Witness-lint and other doctrine analyzers must be resolved before
creating a second owner.

### Provenance

One typed workflow must own:

1. validated repository identity;
2. exact ref resolution;
3. immutable materialization;
4. tree identity;
5. scan policy identity;
6. scanner build identity;
7. produced records;
8. durable artifact;
9. artifact manifest/digest; and
10. independent verification.

The output must not repeat a caller's commit assertion without binding it to
the scanned tree.

### Generic policy input

Product repository names, module paths, exclusions, import catalogs, and
capability matchers must be supplied as validated typed policy owned by the
governance campaign.

The scanner may own generic mechanics:

- safe source enumeration;
- Go parsing/type resolution;
- bounded matching;
- candidate/fact distinction;
- deterministic projection;
- typed violations;
- and evidence finalization.

### Typed output

Replace `write(kind, []string)` with explicit typed record structs or one
bounded typed document.

Every externally meaningful fact must carry its own:

- repository identity;
- source tree identity;
- source path;
- category;
- operation;
- classification as semantic fact or candidate;
- scanner/catalog version; and
- validation.

No record may depend on a preceding record to recover its identity.

### Bounded execution

The specification must set and hostile-test ceilings for:

- manifest bytes;
- repositories;
- source files per repository;
- source bytes per file;
- total source bytes;
- directory depth;
- imports and AST nodes per file where enforceable;
- effects, exclusions, and violations;
- retained memory;
- output records and bytes; and
- wall-clock/cancellation behavior.

Scanning should be streaming with bounded partition state or explicit bounded
spill. A whole-world in-memory inventory requires an explicit, justified
ceiling.

### Semantic honesty

The output must distinguish:

- syntactically proven direct references;
- type-resolved operations;
- unresolved receiver candidates;
- excluded generated/build-specific code;
- wrapper/manual-review requirements; and
- scan failures.

Pure temporal constructors must not be labeled raw clock reads. Generated and
build-constrained source policy must be explicit.

### Process proof

A retained command needs real built-process tests for:

- canonical success;
- malformed manifest;
- represented violation;
- broken stdin;
- broken stdout;
- stderr failure where testable;
- trailing argument refusal;
- cancellation;
- exact exit classes;
- no partial artifact being accepted as final evidence; and
- zero secret or host-path leakage beyond the declared evidence.

Native platform proof is required for every platform claimed by the
specification; cross-compilation alone is insufficient.

## Decision rationale and conditions

### Required hostile proof before any admission

No replacement should be admitted until tests prove:

1. a manifest commit that does not match the materialized root is rejected;
2. swapped repository roots are rejected;
3. a dirty checkout cannot masquerade as a clean asserted commit;
4. ref verification and materialization are bound to the same commit object;
5. source replacement between enumeration and parse cannot escape the root;
6. symlink, junction, mount, and platform reparse-point behavior is explicit;
7. exact source-byte and total-byte ceilings reject boundary-plus-one;
8. file-count, effect-count, violation-count, and output-count ceilings reject
   boundary-plus-one;
9. cancellation at stdin, materialization, walk, parse, sort/spill, write, and
   finalization returns within a bounded interval;
10. imported-name shadowing cannot create a false direct fact;
11. nonstandard package names cannot hide a direct fact;
12. unrelated same-named methods remain candidates or disappear after type
    resolution;
13. generated and build-constrained sources follow the specified policy;
14. every catalog category has positive, negative, neutral, and boundary proof;
15. every output record is self-contained and validates after decode;
16. reordered, duplicated, truncated, unknown, and type-wrong output records
    are rejected;
17. the typed source projection, artifact manifest, and durable bytes agree;
18. artifact replacement is atomic and preserves the previous accepted
    artifact on failure;
19. a fresh five-repository regeneration is byte-identical when inputs are
    unchanged;
20. a one-byte source, catalog, scanner, or repository-identity change changes
    the sealed evidence identity;
21. the real executable rejects trailing arguments;
22. broken stdout cannot produce a successful exit;
23. represented violations remain available as evidence but cannot be mistaken
    for a certified artifact;
24. native platform runs establish all claimed filesystem semantics;
25. race, shuffle, vet, staticcheck, doctrine lint, complexity, and leak gates
    pass; and
26. the data-flow inventory covers every production carrier.

### Recon implications

**Do not admit the July 27 `cmd/capabilityinventory` package into Primitive
2026. Retire the production executable after preserving this interview and the
historical artifact evidence.**

The archive succeeded at its immediate job. Its exact five-archive output
regenerated byte-for-byte, the artifact digest matches its Core ratchet, the
scanner represents failures instead of hiding them, ordering is deterministic,
and the unit-level hostile proof is substantially better than typical
one-off migration tooling.

Those strengths do not establish a supported 2026 command:

- its repository domain names Primitive's products;
- every consumer pin predates the package;
- no consumer invokes it;
- the only located workflow is the completed 2026 freeze campaign;
- there is no command specification;
- Git provenance is manual;
- output is an implicit `[]string` protocol;
- source and memory use are unbounded;
- cancellation is absent;
- file identity is not held across parse;
- semantic limitations remain;
- the actual CLI envelope is untested; and
- evidence production and digest ratcheting have separate owners.

The clean ownership decision is:

1. preserve the checked-in 2026 artifact and this reconstruction as historical
   evidence for the package interviews;
2. do not recreate the executable merely because it existed;
3. do not migrate Kernel, Witness, Bug, Peachfuzz, or Off Grid Software;
4. remove command-specific historical facts from production Core when the
   broader 2026 governance-document decision permits; and
5. only if a named recurring campaign remains, write a new private governance
   tool with compiler-owned provenance, bounded execution, typed self-contained
   evidence, and an independent verifier.

No compatibility layer, package implementation, consumer change, commit, or
push is authorized by this interview.

## Independent review

Fresh independent review of this current report is pending. Earlier drafting or
review statements do not count as independent current-file evidence. This
report is complete as recon; it is not independently approved.
