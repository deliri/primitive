# Version upgrade — v2026.1.56

Tag.UnmarshalJSON previously materialized an arbitrarily large JSON string and
only then refused invalid release coordinates. The decoder now trims legal JSON
whitespace without copying and bounds the remaining token before calling the
shared Core JSON decoder. A tag has one prefix, three uint32 decimal coordinates
and two separators: at most 33 ASCII bytes. A JSON token can escape each of those
bytes into six bytes, plus two quotes, so 200 token bytes exhaust the nominal
representation. This is a coordinate bound, not a document or transfer quota.
Two MiB of surrounding whitespace and fully escaped maximum coordinates remain
accepted. ParseTag and UnmarshalText also bound the nominal representation before
conversion. No second JSON grammar or compatibility adapter was introduced.

The caller already owns the []byte/string input. Parsing adds constant bounded
memory; scanning surrounding whitespace remains linear in that whitespace.
Version has no reader, transport, durable writer, ledger or product policy.
FromProject validates a caller-owned Compass project and derives coordinates;
Core remains the canonical uint32 coordinate parser and ordering owner.

## Boundary inventory

| Surface | Local proof |
|---|---|
| FromProject and Release.Version/String/Tag | real typed Compass construction, exact derived facts, invalid Compass and zero-release refusal |
| Release.Compare | major/minor/patch precedence, equality, extreme values, both invalid operands and typed unknown result |
| ParseTag and Tag.UnmarshalText | shared FuzzTagSemanticClosure executes both public doors, independent strconv coordinate oracle, exact nominal result and unchanged populated receiver on refusal |
| Tag.UnmarshalJSON | FuzzTagJSONSemanticClosure uses bounded standard-library JSON decoding plus independent coordinate admission; valid typed production seed, round trip, canonical second write and typed refusal with preserved receiver |
| Tag.Validate/Release/String/MarshalText/MarshalJSON | exact projections, zero refusal, text/JSON round trips and nil receiver contracts |
| Nominal memory ownership | TestTagJSONNominalBoundPrecedesMaterialization proves the allocation-order structure; TestTagJSONBoundaryLayerTriad proves maximal escapes, unconstrained whitespace, absent input, malformed grammar and preservation |
| Production carriers | existing compiler-visible Release/Tag data-flow inventory; no new production structs |

These are thin adapters over Core's coordinate parser, not a producer/classifier
pipeline. No classifier matrix or durable evidence layer is claimed. The parser
oracle bounds its own work: at most four split components and a 200-byte JSON
token. No raw JSON success seed substitutes for the typed production marshaler.

## Red/green and execution accounting

Evidence lives on furnace under
`/work/engineering-evidence/primitive/version-upgrade-20260910`.
The original clean revision is 039df55d002b1b3a4611fd2541d528fdf5df6363.
Each attempt retains revision, dirty source snapshots/hashes, complete argv,
toolchain/environment, output digests, exit and emitted Go counts. Cache reuse is
disabled for recorded tests, benchmarks and fuzz runs. Missing historical
not-run/unavailable denominators remain unknown, never passed.

allocation-order-red fails the structural guard against original production;
green passes with the bounded decoder. extent-mutation-red lowers the token
ceiling by one and fails the fully escaped maximum test. The corrected
receiver-semantic-mutation-red compiles but zeroes the accepted receiver, and
fails the actual JSON fuzz callback. Both mutations are discarded. The earlier
receiver-mutation-red produced an unused-variable build failure and is retained
as a failed mutation attempt, not semantic evidence. baseline-ratchets also
retains the test author's zero-major Compass fixture error; the fixture was
corrected to an admitted project before later runs.

The fixed 1 MiB invalid-token benchmark runs for 30 seconds before and after,
with the same typed seed, workload, toolchain, host and single CPU setting.
CPU/memory profiles and binaries are retained. It measures refusal allocation,
not throughput over consumed bytes. Final committed checks cover race tests,
release coordinates, lint, vet, staticcheck, strict errcheck and module build;
the final phase then runs the benchmark and each of the two fuzz targets once
for 10 seconds. Exact results reside in the external execution records.

No full-module test acceptance or independent execution acceptance is claimed.
Earlier unrelated Core/AWSidentity/Deploy test failures remain separate.
Source-byte verification and a local manifest check are author evidence.
Consumer module migration remains pending and separate.
