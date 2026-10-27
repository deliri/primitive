# Borrow native copied-field origins

Witness's alternative Temporal admission regression exposed an existing query
gap: `item := parent.Child; return item.Inspect()` returned source-present but
unpropagated, even though Go retained the exact field value and its returned
error. Five focused native fixtures reproduced the gap before the fix.

| Material assumption | Evidence | Status / consequence |
| --- | --- | --- |
| A local copy necessarily loses the selected field's value | Actual Go SSA source references retain the same loaded value | REFUTED; borrow those references instead of reconstructing assignments |
| An incoming record parameter always stays in a bare SSA parameter | Retained native frame dump shows a compiler local home and field-address loads | REFUTED; the first attempted direct parameter comparison stayed red and is retained |
| Same spelling identifies the selected field | Native `types.Var`, field selection and borrowed SSA values identify it | REFUTED; shadows, neighbors, replacement values and different target methods remain unproven |
| A whole-record copy can lend its field after any intervening write | Existing `receiverGateHomeStable` rejects intervening writes and opaque escapes | REFUTED; retain the actual source value, native store/home and read; disabling that guard must make the replacement control fail |
| A later write replaces a previously loaded value | Native loaded snapshots are immutable | REFUTED; checks cover loads before later replacement |
| These finite fixtures prove all possible aliases or runtime paths | The API remains an observed call/error-flow floor | UNVERIFIED; no whole-program or runtime alias claim |

The change stays in Primitive. It reuses existing native instruction loans,
`ArgumentOperand`, source-value lookup, and record-home stability. There is no
assignment history, alias registry, graph, source population, quota, or mutable
state machine. Witness continues to borrow the same public typed observation.
Working storage for these loops is fixed; existing native field-depth/call
ancestry limits are not promoted into a universal constant-memory claim.

Base is Primitive420 `c2dda80ac01de3319c527d2aa33b1d186d75d2d1`.
All development requests use installed Mac Anvil, explicit darwin-arm64,
CPU1/RAM4GiB, a retained source snapshot, race, shuffle and count=1. Raw requests,
receipts, output, discarded faults and profiles remain outside Git in the owned
Witness scratch root. Readback checks artifact hashes/extents and test events.

- Original native red: `2026-10-27-f8d708de1f52a62aba29d715a4cb4fe04003d358324f243fcb010964308fad23`,
  snapshot `ae896d568503c403e34dc98698706508d8d6c6a221ac3597c93e17593b459abd`:
  all five copied-field/parent/helper/snapshot leaves fail (6 nodes with parent).
- First parameter-only attempt `2026-10-27-6f95334bd60b36acffc1812c58a436d7c187ed23fa7cc36e67680de2d4564579`
  stayed red at all five leaves. Focused native frame diagnostic
  `2026-10-27-f2fa65e714fa66b1a749927923d2dc2fc037b8cec7bb0028483240d26bf8dcf3`
  retained the actual compiler lowering. Neither attempt is counted as success.
- Fixed exact five-leaf retry `2026-10-27-4d9284b14b3277dc8c0db3b9f78bd87e16cc5b2828fa708a76a26e61793f7151`:
  6 passed nodes, original seed8965405095750095637. Snapshot
  `4f48c2aae3b135f7a01616a32c2f8dc95f6e10682dfe633580336a76acbcdab6`.
- Remaining changed/affected field, cause, container, origin, custody,
  cancellation, constructed-slot, allocation and fuzz-seed checks:
  `2026-10-27-ef399055c0f93bd1c691b2e6e0c384a85cd5bb89ca5651727f3fd02b848bf30b`,
  85 passed nodes, package9.939s. Final source/test snapshot
  `79f2f041c508756c0a4f98d55b3cd83ac17c6930156c675ea1e4a7d129b27080`.
  The existing native query allocation check measures3512bytes at both32 and
  16385 syntax statements, excluding compiler construction. This is a result
  for that workload, not a universal runtime/memory bound.
- Disable copied-home stability: `2026-10-27-052c4b4a105a12b0eac88a2316548136c5ec54136de7ec54abe0158495edcbfc`
  fails at `copied_entire_parent_replacement_loses_field` with unearned true.
  The deliberate fault is discarded; its exact failed-leaf restored retry and
  final tool/measurement evidence are recorded below after verification.

This is development verification. Witness's other findings and independent
acceptance remain open.

## Final scoped verification

Unmutated final source/test snapshot is79f2f041 above. The restored stability
leaf `2026-10-27-0159ce6202584a43729e281a17399e901f7acc212ad20e4cdb88f18dd9a59604`
passes2 nodes at original seed186377825319455064. Its isolated pass preserves,
rather than erases, the original controlled failure.

Scoped Go vet `2026-10-27-335df779f5fa939ce44f92acdc2a725e8bd730b6336f2f99282a98923e91f5b3`
and Staticcheck `2026-10-27-0cb539d7cf59b5713323fc7793d918d35a0ed496300c1809de110481674e6135`
each execute once, native0, zero findings. Raw analyzer streams also verify
against receipt hashes/extents and contain zero analysis-output bytes.

Serial profiled three-second benchmark
`2026-10-27-1a2a8f2354eab4223e121d4a9ae35177abed73b6da71f273d0348fd3244a937f`
contains one actual measured row:272667iterations,12779ns/op,3208B/op,
56allocs/op. Native row count equals the retained measurement count. CPU profile
16238bytes SHA2564e7e4ac0efbd52a1e5c07e4c8a9927fb0fde9f762dd7f0176f4ce18f8098eeaf;
memory profile3805bytes SHA256598a715c88433972919d6f0b402d9f331492250ac2a677fea137855f4066c6f5.
Profiles are retained outside Git. Profiling perturbs measurements; no matched
performance improvement or retained-heap bound is claimed.

Serial three-second semantic fuzz
`2026-10-27-771ded11d323597dad8ee1cdc24bc0c2f23ec3649db32f09e566ee71f6cb262d`
passes with native counters36executions and5new-interesting inputs. These are
native counters, not unique-mutation proof or sustained independent acceptance.
The seed oracle now includes copied selected and neighboring fields alongside
its existing target/cause/constructed-slot boundaries.
