# Gomodule review — v2026.1.66

This slice pins the public module/import identity boundary. Production grammar
and API are unchanged: Go's x/mod owns the language, and Primitive owns nominal
custody, typed refusal, and exact canonical scalar projection. Baseline commit
is d2763d6807b152b3b264843b6c62891d770a9798.

## Evidence surfaces

- ParsePath and ParseImportPath each have a direct text fuzz target. These prove
  x/mod admission parity, exact input identity, zero refusal output, the
  core.ErrGoModuleContract wrapper, and preserved module.InvalidPathError facts.
  The upstream comparison proves wrapper fidelity, not an independent proof of
  x/mod itself; named hostile grammar cases supply the independent examples.
- Both UnmarshalJSON fuzz targets independently decode source text with the
  standard JSON implementation, check acceptance against the owning grammar,
  compare the exact admitted identity, and prove canonical closure. Refusals
  preserve a populated receiver and leave a fresh receiver zero.
- Local JSON layer triads cover positive source identity, typed syntax/domain
  refusal, and absent/null/zero behavior. Nil receivers refuse safely. Named
  representation rows include escaped separators, Unicode escapes, traversal,
  invalid UTF-8, unpaired surrogates, truncation, trailing documents and wrong
  JSON kinds. No filesystem, durable writer, ledger, or classifier exists in
  this package, so those layer and producer/classifier rules do not apply.
- A narrow AST inventory binds all public constructors/methods, four external
  doors and their fuzz targets, and both production structs to intentional
  sealed-identity roles. The live matcher is also exercised on synthetic Go.
- Duplicate quota rows and 10/10/20 counters are retired. Four lengths around a
  deleted 1 KiB ceiling are not four boundaries. One 2 MiB admitted fixture now
  ratchets absence of that historical cap. These adapters do not own a second
  serious parser/validator implementation; copying upstream grammar branches or
  padding the wrapper tables would weaken the evidence.

## Red states and execution

This is a contract ratchet over already-correct production. For each nominal
identity, replacing decoded source with an unrelated valid constant causes the
JSON fuzz seed oracle to fail. Replacing the upstream error with a core-only
refusal causes the text fuzz seed oracle to fail. Adding an unclassified
production struct causes the inventory to fail. All five deliberate mutations
are recorded and discarded; original production bytes are restored.

Evidence lives at /work/engineering-evidence/primitive/gomodule-upgrade-20260911.
The recorder retains committed revision plus dirty facts and exact source bytes,
commands, cache posture, toolchain, outputs, exit statuses, attempts, and hashes.
The final gate discovers the complete compiled package test/fuzz/benchmark
scope, runs uncached race tests for Gomodule and Compass, all package analyzers,
and a full module build. Benchmarks run once per 30-second pass with CPU/memory
profiles and retained binary, followed by serial 10-second fuzz budgets.

The benchmark loop and identity workloads remain the same. A pre-timing fixture
check and byte metric now make workload size explicit. Baseline and candidate
samples remain retained; no performance improvement or machine-isolated
comparison is claimed. Machine power posture is unavailable.

## Memory and limits

Text admission walks the caller's existing Go string using x/mod and retains
that immutable nominal identity; it does not collect a graph, project tree,
stream, or additional collection of path components. No arbitrary size ceiling
is added. JSON methods necessarily materialize their returned Go string/byte
slice. Their nominal output occupies O(identity length) memory; this is not an
end-to-end O(1) bulk-stream API or a claim that a petabyte identity fits in RAM.
Bulk paths remain caller-streamed through the separate GitHub TreeEntryStream
contract. No product policy or product state machine belongs here.

Local package-gate closure is conditional on the final committed run and sealed
integrity report. Independent acceptance, full-repository test success, and
external consumer adoption are not claimed. The previously documented broader
repository ratchet failures remain outside this package's execution scope.
