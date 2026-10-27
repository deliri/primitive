# Argument.Value owns its native receiver admission

| Assumption | Evidence | Status / smallest consequence |
| --- | --- | --- |
| A member named Value proves receiver validation | Exact native process.Argument.Value calls Validate; lookalikes remain unadvertised | REFUTED; advertise only the reviewed owner/receiver/member tuple |
| Package membership grants the contract to other receivers | EnvironmentValue and package-function controls refuse authority | REFUTED; preserve exact tuple identity |
| Empty and unset arguments mean the same thing | Constructed empty succeeds; zero receiver refuses with ErrProcessContract | REFUTED; prove both actual native states |
| Native argv values are necessarily UTF8 or shell syntax | Existing Argument contract preserves arbitrary non-NUL bytes | REFUTED; verify exact bytes without executing them |
| Successful projection promises successful OS execution | Value validates only the existing Argument domain | UNVERIFIED; no process launch, kernel argv limit or execution-success claim |
| Witness needs its own validation call to clear this finding | Primitive's existing ValidatesReceiver authority can express the actual method | REFUTED; extend that one authority, no Witness registry or duplicate gate |

The only production change advertises process.Argument.Value alongside the two
previously reviewed receiver alternatives. Core.PackageProcess owns its package
path. The native method, producer, limit and refusal semantics are unchanged.
No public API, type, quota, executor, registry, state machine or generic Value
rule is added. An unadvertised valid symbol remains false/nil; malformed symbol
ingress retains the existing Core refusal.

Tests bind the actual native method pointer/type, execute unset/constructed
empty/spaces/opaque bytes/inert shell syntax/NUL and below/at/above the existing
1MiB ArgumentMaximumBytes boundary. They require exact admitted bytes and
ErrProcessContract/zero outputs on rejection. Seven distinct namespace/member/
receiver controls prevent authority borrowing. The existing semantic namespace
fuzzer now checks all three reviewed baselines on every mutation.

Base9c8f805d21ca527bf40180b586b7ac28babfc300 / published Primitive428, preserved
from the concurrent publisher. Mac Anvil, JSONv2/race/shuffle/count1,
CPU1/RAM4GiB. Raw requests, snapshots, receipts/output and corpus observations
stay outside Git in /private/tmp/witness-native-unblock-z5l6ve72.

- Original native red2026-10-27-f90e40ace4d033315a4be14d21017d7aaed773261cd10260dd0f99febc559df2,
  sourceb8b9b07ef3b635fc4b39fd4896fd577293484f3524926937bfeee2b41ac99efe:
  ten failed nodes, nine boundary leaves and their parent. Native value/cause
  checks complete before the missing advertised contract fails.
- Exact nine-leaf retry2026-10-27-544c2cb961c4a666867865719cdd2a067894dfb9ec9559e40388b477d2466fe5:
  ten passed nodes/no skips, original seed8973176338217703832. This is a batch,
  not an isolated-pass or original-order claim.
- New lookalike controls, prior Temporal/ByteCount boundaries and semantic seeds:
  2026-10-27-93cc8008bcb6086acbd0500688ea45d68f7a93dde73c63d85f4a5ffe092baebe,
  45passed nodes/no skips. Both bind sourceeeca0ba3c48bb0d6258d3c4150ca9b1b76f485d7745f86d356f310a60ff7c3d5.
- Scoped Go vet2026-10-27-57978cda1ffd7a5dd3132659014f3cf7dd9360ba398e75e6d77ba819a9a8e358
  and Staticcheck2026-10-27-39c0fdab47242f09d9442676705f1196ff37b502c48361be7494263ff145f898:
  one actual execution each, native0/zero findings, same eeca0ba3source.
  Receipt-bound raw analyzer streams independently verify zero output bytes.
- Serial3s namespace fuzz in the Primitive phase, after its tests/tools:
  2026-10-27-92c6450cab194562e2daa66477c2f6aeb6941bc12a5d5baf6b7b4d28e6d8656c,
  one passed target; native baseline10/10 including cached inputs,
  11reported executions/0new-interesting inputs. These are finite development
  counters, not unique mutation or independent-acceptance claims. Independent
  Witness recovery continued under separate Anvil resource admission; no
  isolated throughput or benchmark result is claimed.

All raw correctness artifacts have independent digest/extent and native event
accounting verification. The source after the original red changes only the
admission fact, fuzz baselines and failure diagnostics. No passing campaign was
replayed for timing. Final fetch/version choice, publication, official Witness
adoption and the actual compiler-backed consumer regression remain separate
steps. Independent acceptance belongs to the user.

Final fetch retains origin/main9c8f805d unchanged; release429 is unoccupied.
The final authored proof/comments and release number add no executable change.
