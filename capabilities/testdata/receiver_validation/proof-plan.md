# Reviewed receiver admission

The consumer is Witness's native child-delegation classifier. It currently
demands a direct `Validate` target even when Temporal's `NumericInstant.Instant`
has already admitted the receiver and returned its typed refusal. Adding a
second validation call in Witness Core would duplicate work to satisfy a rule.

| Claim | Evidence | Status | Consequence / smallest check |
| --- | --- | --- | --- |
| `NumericInstant.Instant` refuses an unset receiver and preserves Temporal's contract cause | `temporal/numeric.go`; actual method-expression runtime test | VERIFIED | Advertise this exact declaring method only; delete its validation branch as a causal control |
| Every signed Unix-nanosecond instant is valid, including epoch and pre-epoch | Temporal's constructor and numeric projection contract | VERIFIED | Check minimum, -1, epoch, +1 and maximum without adopting a consumer's post-epoch policy |
| A valid method identity proves that a caller uses it correctly | No such fact follows from a catalog lookup | UNVERIFIED; explicitly not claimed | Witness must separately request compiler-bound field/call/error propagation from `gotoolchain` |
| Method spelling or package prefix establishes receiver validation | Counterexamples: foreign owner, another receiver, package function, presence observation, unadvertised methods | REFUTED | Single-symbol mutations must remove the grant; invalid symbols retain `ErrCapabilitiesContract` |
| Any other Primitive method has the same admission contract | Not reviewed by this slice | UNVERIFIED | Return false/nil, retaining absence of knowledge; do not infer from method names |

`StandardSymbol.ValidatesReceiver` adds one pure lookup over an existing typed
symbol, without execution, source traversal, state, allocation proportional to
the input population, new protocol types, or a second package catalog. Temporal's
import path comes from Core's existing architecture authority. No Temporal
production behavior changes in the release.

The upstream lookup has one admitted symbol and no multi-fact producer/fold or
contradiction precedence: manufacturing a 50-row quota for that lookup would
not add proof. Tests connect the advertised fact to the real compiled Temporal
method, exercise its success/refusal boundary, preserve typed symbol admission,
and mutate each identity coordinate. The consumer's native producer/classifier
handoff still needs its existing hostile delegation checks plus the imported
alternative-method boundary; this upstream result alone does not clear it.

Raw attempts, exact snapshot digests, requests, receipts and mutations live in
the owned SSD scratch root, outside Git. Record verified run identities below
after readback; passing development checks do not issue independent acceptance.

## Retained development proof

Base: Primitive `a0be3e240d984a708bea300e52a5c3ac494d7c82` / 419.
Unmutated production/test source snapshot:
`fec918ac6c08a9cf6403b99742607392dc0317e41e6cb3ca5285e0558d291020`.
All requests use installed Mac Anvil, explicit darwin-arm64, CPU1/RAM4GiB,
source snapshot, race, recorded shuffle, and native count=1. Raw receipt and
stdout/stderr extents/digests and test-event accounting were checked independently.

- Focused methods, hostile symbols, three fuzz seeds and existing inventory:
  `2026-10-27-f0646a28f2c34a53fa695e021ef6b20494b73b17244ac21a47ac3bb29e5e1b81`,
  25 executed nodes, all passed, package1.457s.
- Remove Temporal's actual unset refusal in an isolated owned worktree:
  `2026-10-27-4038164800aaf665892d65d8a3b1f15ce17829af84478c9d0f0e221fcb9a49cc`,
  red at `MatchesTemporalAdmission/unset`. Restored exact failed leaf/seed:
  `2026-10-27-b1d515a279e45488cf827da785f2acc147aec0cdf964794373b5c832b02433bf`,
  2 passed nodes, seed2313736557813922610.
- Remove the catalog's namespace binding:
  `2026-10-27-3b553abb259e977cfa3d0a4604e1634951a198ebc1bda0075c347b09c2c0948f`,
  red at `DoesNotGrantLookalikeContracts/foreign_namespace`. Restored leaf:
  `2026-10-27-8160a1149aabee1ee9d1717d479c0ef79bc0133f12e76710faf53f6716e80f70`,
  2 passed nodes, seed2137693998361299902.
- Remove typed symbol admission:
  `2026-10-27-1b7f963bf30aef820f8432b6c4482bbde361fd3fc291acf7e06c9e4dff771323`,
  red at `DoesNotGrantLookalikeContracts/invalid_selector`. Restored leaf:
  `2026-10-27-322702919bdfe3fa3f0380256f7ebb858a8283c112477b379842cf89fa4d9fe7`,
  2 passed nodes, seed990733772645562305.
- Scoped Staticcheck `2026-10-27-847084238a705257a5c7f8c5d167c789c08f9bad922417f8cf6edd2d4eb21a00`
  and Go vet `2026-10-27-49e83e766f2f8126c08c938c7c5877744c9e471cd1adc910e390307ac3839ffe`:
  each 1 actual execution, native0, zero findings. Retained analyzer streams also
  match their receipt hashes/extents and contain zero analysis-output bytes.
- Serial three-second semantic fuzz:
  `2026-10-27-33d4ca8a0a46586655ac55485f0496775e9a0d6367477a508282536afb0a4f91`,
  passed; native reports368 executions and6 new-interesting inputs. These are
  native counters, not independently proved unique mutation counts.

All deliberately injected production faults were discarded. The release adds
only the capability lookup, tests and this authored proof account. No native
Temporal behavior is changed, and these checks do not clear Witness's open
self-lint findings or issue independent acceptance.
