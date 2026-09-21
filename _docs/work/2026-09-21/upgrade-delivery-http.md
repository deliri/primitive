# Upgrade delivery HTTP regression

The live installed Peachfuzz upgrade received an empty HTTP 200. API source
4391169 ignored the error from exchange.WriteJSON. Primitive v2026.1.96 could
marshal UpgradeDeliveryProjection but its exchange encoder attempted to decode
an issue-only bearer into the issuing type. The missing receiving-projection
contract is now supplied through the existing typed validation helper.

The retained red test executes exchange.WriteJSON with genuinely signed
fixture agreements and fails on f76bc45 plus only the new regression test.
The green test checks exact HTTP bytes, content type, and both independent
signature/binding verifiers. Missing download, missing transfer, and zero
projection emit no partial response. The existing semantic fuzzer now also
pressures the issuing projection against mutated bytes.

Scope: distributionauth only. Full package uncached tests and race/shuffle
count=2 passed; semantic fuzz budget is 2s plus 1s minimization. Vet,
staticcheck, and doctrine passed. Raw fieldalignment reported one new table
layout finding, corrected; its four remaining findings are exact pre-existing
JSON fixture declarations pinned in scripts/fieldalignment_wire_layouts.json.
No production wire order changed. Execution facts and output digests are in
upgrade-delivery-executions.json; complete logs remain at the recorded paths.
These are development execution facts, not independent acceptance. The
recorder does not enumerate excluded scope or classify all timeout/unavailable
outcomes; those accounting fields remain explicitly incomplete.

API response error handling and consumer HTTP regressions are separate product
changes. This fix does not complete interrupted upgrade recovery, candidate
behavior reporting, or connected GCS/Firestore diagnostic delivery.
