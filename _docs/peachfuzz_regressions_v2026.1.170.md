# Peachfuzz retained regression repair

Peachfuzz's signed current/actionable queries for Primitive recorded 65 current
findings: 54 no longer reproducing and 11 reproduced inputs across seven fuzz
targets. The exact 11 inputs also failed against published v2026.1.169 on
Furnace. These findings exposed incorrect semantic assertions and unstable
signed test fixtures; this slice does not claim eleven production defects.

The repaired contracts retain the actual production verifiers:

- Cloudflare upload responses compare the endpoint with the standard-library
  URL projection, retaining provider identity and exact escaped URL facts.
- Image delivery preserves the caller's validated origin, including a comma in
  its host, while requiring the exact image path and escaped path delimiters.
- Upgrade delivery accepts the owning verification or binding refusal, retains
  the distribution contract identity, and requires an invalid returned proof.
- Publication completion preserves distribution binding refusals as well as
  attestation/control-plane refusals and requires an exactly zero result.
- Signed release/publication fixtures pin historical compiler provenance.
  Their expected signed documents no longer change with the toolchain selected
  for new builds. Previously emitted Go 1.27.2 seeds remain explicitly signed
  typed expectations; unrecognized authenticated recombinations still fail.

Eleven regression seeds are authored under the existing target corpora. Two
publication inputs had a roughly one-megabyte whitespace prefix; only that
prefix was removed from the promoted seeds. Their exact original bytes remain
on Furnace, and both originals were included in the eleven-input green replay.

Retained execution evidence:

- `/data/evidence/primitive/2026-10-08/media-projection/peachfuzz-actionable169-red01`:
  all 11 original inputs failed; 18 failing test events include seven parents.
- `peachfuzz-actionable169-diagnostic01`: original diagnostic replay retained.
- `peachfuzz-actionable170-green01`: ten inputs pass, one request remains failing.
- `peachfuzz-actionable170-diagnostic01`: the remaining historical module-mode
  difference is retained; it exposed a fixture-edit mistake, subsequently fixed.
- `peachfuzz-actionable170-green02`: all 11 original inputs pass, 18 passing
  events, zero failures/skips across three packages, 2.203 seconds process time.
- `peachfuzz170-packages01`: complete cloudflare/distributionauth/release scope,
  2167 passing events, zero failures, one explicit skip, 11.353 seconds.
- `peachfuzz170-race01`: the same complete scope under race/shuffle, 2167
  passing events, zero failures, one skip, 40.594 seconds. The skipped live
  executable witness requires the admitted Darwin/ARM64 build host and was
  not executed on Furnace's Linux host.
- `peachfuzz170-vet01`, `peachfuzz170-static01`, `peachfuzz170-build01`: pass.
- Seven `peachfuzz170-fuzz-Fuzz*-01` runs pass, 5420 total executions. Each
  uses a two-second search budget and one-second minimization budget. Receipts
  separately record startup/replay/shutdown overhead: 3.475–14.050 seconds.
- `peachfuzz170-doctrine01` retains 68 Cloudflare findings. Exact published
  v169 comparison retains the same 68 file/rule/line/column identities, with
  no additions or removals. Distributionauth/release checks pass with zero
  findings. The first two baseline setup failures remain retained; the root
  scan that emits no findings is not substituted for per-package comparison.
  Comparison source and outputs are under
  `/data/evidence/primitive/2026-10-10/peachfuzz170-doctrine-comparison01`.

The original-input and promoted-input hash manifest is retained on Furnace.
Execution receipts identify revision, dirty-source facts, commands, toolchain,
counts and output hashes. Candidate proof is not committed-source proof or
deployed consumer proof. Full Primitive verification, downstream integration
and independent acceptance are separate surfaces.
