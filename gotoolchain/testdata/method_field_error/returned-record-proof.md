# Borrow returned field origins

Witness's inventory getter and TSA receipt projection preserve child values
and child refusals at runtime. The existing compiler query misses the returned
value between the getter and the consumer. Witness must not duplicate its
validation calls or keep a getter allowlist to satisfy that query.

| Assumption | Evidence | Status / consequence |
| --- | --- | --- |
| A getter's name proves field provenance | Neighboring, fresh, opaque and overwritten returns do not retain the selected value | REFUTED; borrow the native producer, result ordinal, parameter and field slot |
| A native parameter always remains a bare SSA parameter | The first parameter-only attempt stayed red; Go gives these records local homes | REFUTED; reuse existing ArgumentOperand and stable stored-record proof |
| A tuple's first result can stand for another result | The selected and neighboring tuple controls disagree | REFUTED; preserve Go's exact result ordinal |
| A getter read proves its return | Unused reads and fresh returns stay unproven | REFUTED; inspect the actual returned expression and native operand relation |
| A valid early slot can conceal damaged later compiler keys | Missing/copied/neighboring keys refuse with Core's existing output error | REFUTED; validate all native keys before lending a constructed slot |
| A field-return query proves every runtime path or alias | This API remains an observed call/error-flow floor | UNVERIFIED; no whole-program or universal alias claim |
| Source extent requires a retained syntax population | Direct and returned queries have fixed measured allocation at both source extents | REFUTED for these workloads; compiler storage and active ancestry remain separately owned |

The implementation extends the existing private binding query. It borrows
ExpressionCall, native source declarations, source values, ArgumentOperand and
the existing record-home stability guard. Call bindings have one shared owner
for helper consumption and returned values. There is no new public API,
protocol type, symbol registry, alias history, state machine or input quota.
Unavailable/dynamic bodies and unsupported return representations grant no
authority. The iterator retains only active bindings, not a return population.

## Verified development attempts

Base7174033b8850e14af15a5216eb090bdd792de326 / Primitive423.
All execution uses installed Mac Anvil, darwin-arm64, JSONv2, CPU1/RAM4GiB,
race/shuffle/count1 for correctness. Raw requests, receipts, output, profiles
and discarded attempts remain outside Git in the owned scratch root
/private/tmp/witness-native-unblock-z5l6ve72. Independent readback checks
artifact hashes/extents and actual Go event accounting.

- Original red2026-10-27-69a740fb1338f5e407f9eba929f7d14fcdf046db45d5d6005d4ae18ac32e4858,
  source2b41fa2c7cbb7747db8b72c073c1e0ad7ab52a38d51b7798d7774da4ae53cb29:
  seventeen nodes, ten pass/seven fail. Six positive getter/tuple/projection
  leaves and their parent fail; negative controls already pass.
- First parameter-only retry2026-10-27-4ff3a1f39f949bdf081c7104b00e40c64f04a0e066d95ba0a0e792c150c6f65e
  stays red at all six leaves. Sourcea45948736d464cb010674e764baee597b3e515ce62eae6480290ac9cc89ee7f8.
- Reused stored-record proof, exact six-leaf retry:
  2026-10-27-7e47a1b446a37c9738aeec9e540fd620136c127e836fe78ccee94f9fd9e6f307,
  seven passed nodes, original seed3806562571098290930.
- Affected ten negative controls:
  2026-10-27-9a8d8bf8150f9c33db9976155cb38ccfc1a7403291ab933506309cec472f7cb0,
  eleven passed nodes. Both bind source37dc3dc83e09be66eb73502e9b6dfc65f0cc5397031236f5674f392dcce0bf3c.
- Existing affected target/helper/constructed-slot/generic/cancellation/source
  contracts, new damaged-return bindings and expanded semantic seeds:
  2026-10-27-adfcfa2a639c35d8838b846c3c4d4408e1e17aecd522c14fadb0760064e86b0b,
  106passed nodes, no skips, sourcefd0a909fecc7cce7968ac6b20f0325ae292c6bc03eea5d2f17a2f3b0927a00bd.
- Actual inventory-style record comparison before validation:
  2026-10-27-c8627e713cd5092073e7ebdd2d3f2c689ebf1e5b8fa4698da38231971a7f37ab,
  two passed nodes. Source980061e38c51c7b02dbd1fd1e185d2cfde2861fb4adeb3c8ccab9d56f3787960.
- Allocation ratchet2026-10-27-82887ba036bfc7051532ad72b6d57197ce6fffc9e949ed7229c780e20cbd6e6c:
  one passed target. At32 and16,385 statements, direct construction allocates
  5,304query bytes at both extents; the returned record allocates4,872 at both.
  Compiler construction is excluded. Source359d0c4a4784ce38fa22ad0a9708190bd950729a542cf842104d837fee946a77.
- First tools plan025eecbd43011aa9c1058f39ba9d8ec314cd32449158412eee8cc033c6d1d03b
  retains vet success and a real Staticcheck S1023 redundant return finding.
  Removing that redundant return changes no query behavior. Final Go vet
  2026-10-27-47085ff2a6cb83b7243cbe89f32452c39cf6a423d8bdee4f3a2117f60d1fdfc1
  and Staticcheck2026-10-27-f1a781bf9180facae7eb541623e498b6b5a7a693d53073626e2fa21444cb7686
  each execute once, native0/zero findings; receipt-bound analyzer streams
  verify zero analysis-output bytes.

Final source/test snapshot26a87fcbeca8155723d2932bafe86a9e15b24233e79861516b20cf5413bd71f4:

- Serial profiled3s benchmark2026-10-27-3c21e6106c67b589fbfd55404163f657f53d132356564137304e064da235857d:
  one actual measured row,95,794iterations,39,859ns/op,4,400B/op,80allocs/op.
  CPU profile13,239bytes SHA2560bd2a6ccba7fb60a9562f409f69c12b3d3ee2d14d537e115903dcdf1d370e9eb;
  memory profile5,353bytes SHA2567a360e8b4c416becee6171ba2fafcf5aa9842e64765f7e61cf307d9dfc2dc32a.
  Both retained profiles and the raw measured row independently verify.
  Profiling perturbs timing; this is not an isolated fleet speedup claim.
- Serial3s semantic fuzz2026-10-27-9ae0e7770f6b34e8ceff5319d83b4fb954c8cc8b730b3b52b91b971b2c12b04b:
  one passed target; native baseline25/25 including cached inputs, reported
  28executions/0new-interesting inputs. These are native counters, not unique
  mutation proof or sustained independent acceptance.

During verification another publisher released424 at3107edab, changing only
installation.go, its existing module-file test and the release number. That
published work was fast-forwarded into the owned branch without overwriting
either set of files. Getter integration on the combined source is recorded
after readback, before choosing the next unoccupied release number.

Combined-source integration:
2026-10-27-cfd331be54aab0ff96d7ec2a5a7ce533ef2a00e81f5bcc9c6340939d4e024a71,
base3107edab833b0afd89989f85de983d6fc94f42bc / published424,
snapshot47243a6ef32a6be4f2e1fc160f87e83c65580ee1848e3f80c23c7193286745eb:
eighteen passed nodes, no skips, including the added record-comparison path.
Raw receipt/output hashes, extents and event counts independently verify.
The release number425 was unoccupied at the final fetch. The final combined
scope's analyzer readback is added before publication; other previously proved
query, memory, benchmark and fuzz paths are unchanged by the disjoint stderr
capture addition and are retained under their actual earlier source bindings.

Final combined Go vet2026-10-27-e8282442b937903eabafc77c8c7a9eb1517d7230316dfdb1213368296bc63bd7
and Staticcheck2026-10-27-b466361434f030446d292a336de35036879824f7804dcffd315c9a8fb7bdb9bd
each execute once, native0/zero findings, sourceba728c6c0167ac7ace3543874653376ff7cbc4bf3716a6851801d8883159cd0c.
Independent receipt/raw artifact and analyzer stream extents/hashes verify
zero analysis-output bytes. The final authored proof adds no executable code.

These results do not clear Witness's other findings or issue independent
acceptance. Publication, official module adoption and the actual consumer
classification remain separate proof steps.
