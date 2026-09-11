# Primitive upgrade priority — 2026-09-09

Published release: **v2026.1.35**. See [release notes](release_v2026.1.35.md)
and [the Distribution review fixes](distribution_review_20260909.md).

**Current requirement applies to every package:** fixed memory windows for
reads and writes, with no arbitrary input/output extent quotas. The new
[streaming audit](streaming_audit_20260909.md) inventories all 63 packages.
Previously completed slices remain historical; none is exempt from this audit.
Lineio is currently being changed to fixed-buffer fragments. Witness source
migration is user-owned; the installed witness-lint still checks this slice. Its older whole-line measurements are retained as historical evidence.

## Completed slices and separate follow-ups

These packages completed earlier quality sweeps; they remain in the new streaming audit:
Attest, AWSidentity, Capabilities, Exchange, Filestore, Temporal, Hostfacts,
Process, Core, Contextstate, Controlwire, Keygen, Release, Objectstore, Chit, Controlplane, Shutdown, Currency, ID, and Distribution.
Individual reports live with those packages or under `_docs`. The first three
have September 6 upgrade reports. Exchange/Filestore were released in v2026.1.19;
Temporal/Hostfacts in v2026.1.20; Process/Core were checkpointed in v2026.1.21;
Contextstate/Controlwire in v2026.1.22; Keygen in v2026.1.23; Release in
v2026.1.24; Objectstore in v2026.1.25; Chit in v2026.1.27; and Controlplane
in v2026.1.31; Shutdown in v2026.1.32; Currency in v2026.1.33; ID in v2026.1.34. The separate v2026.1.30 Manual grammar change is a scoped addition,
not a completed package sweep.

GoToolchain was integrated in v2026.1.21 and is reserved for an explicitly
requested later quality pass. GCSobjects received a scoped streaming repair in
v2026.1.26, not a complete package sweep; keep that follow-up separate. Historical
review limitations remain historical evidence. The new streaming requirement
authorizes revisiting these packages, one at a time. The September 7 usage snapshot below is unchanged;
it measures import breadth and does not itself identify remaining work.

## Evidence and ranking method

Go's AST parser inspected 6,210 source files across Primitive and the four
consumers, with no parse errors. The ranked measure below counts **production
source files directly importing each Primitive package**, once per file. It
excludes tests, vendor copies, testdata, hidden artifacts, examples and auxiliary
underscore/test/evidence directories. Production generated files are included.
All discoverable platform/build-tag variants are included; build constraints
were not evaluated. This measures source dependency breadth, not runtime request
frequency, latency or CPU. Package-selector counts in raw evidence include types
and constants and are not advertised as actual function call counts.

All 1,937 consumer production files were inspected. The source hashes were
rechecked after capture: no files changed during the scan. All five repositories
were dirty; exact heads, dirty path lists, module files, the workspace file and
per-file source hashes are retained in
`.artifacts/consumer-usage-20260907/`. The retained `inventory.go` uses Go AST
imports and resolves import aliases. The raw inventory retains tests and
auxiliary files separately. The table covers 61 discovered non-auxiliary
Primitive package directories; 46 have direct consumer production imports.
Zero direct importers does not mean unused: providers and shared implementation
packages may be reached transitively. The final column records direct Primitive
package importers from the local production graph, not runtime reachability of a
pinned vendor version.

## Direct production import breadth

| Package | Total files | Blink Kernel | Peachfuzz | Bug | Witness | Primitive package importers |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| core | 735 | 363 | 130 | 83 | 159 | 60 |
| temporal | 387 | 210 | 67 | 35 | 75 | 34 |
| filestore | 168 | 50 | 40 | 17 | 61 | 8 |
| hostfacts | 155 | 60 | 21 | 21 | 53 | 3 |
| contextstate | 139 | 53 | 40 | 14 | 32 | 14 |
| exchange | 137 | 110 | 7 | 7 | 13 | 11 |
| process | 80 | 17 | 18 | 11 | 34 | 7 |
| controlwire | 63 | 3 | 20 | 19 | 21 | 14 |
| keygen | 45 | 14 | 4 | 12 | 15 | 6 |
| release | 45 | 0 | 15 | 16 | 14 | 4 |
| objectstore | 44 | 23 | 10 | 6 | 5 | 9 |
| chit | 32 | 0 | 20 | 6 | 6 | 4 |
| attest | 30 | 1 | 12 | 8 | 9 | 16 |
| controlplane | 26 | 0 | 12 | 8 | 6 | 6 |
| gcsobjects | 26 | 26 | 0 | 0 | 0 | 0 |
| shutdown | 24 | 15 | 2 | 1 | 6 | 0 |
| currency | 24 | 24 | 0 | 0 | 0 | 1 |
| id | 20 | 5 | 2 | 4 | 9 | 6 |
| distribution | 19 | 0 | 7 | 6 | 6 | 1 |
| lineio | 16 | 0 | 0 | 0 | 16 | 0 |
| fuzzartifact | 15 | 0 | 9 | 0 | 6 | 0 |
| runprotocol | 14 | 14 | 0 | 0 | 0 | 4 |
| lease | 13 | 0 | 3 | 5 | 5 | 2 |
| retrieval | 13 | 0 | 7 | 3 | 3 | 1 |
| manual | 12 | 5 | 3 | 2 | 2 | 0 |
| receipt | 12 | 1 | 3 | 5 | 3 | 7 |
| payment | 12 | 0 | 6 | 3 | 3 | 1 |
| submission | 11 | 0 | 5 | 3 | 3 | 1 |
| compass | 8 | 4 | 2 | 1 | 1 | 2 |
| googleidentity | 8 | 5 | 1 | 1 | 1 | 0 |
| filelock | 7 | 2 | 1 | 2 | 2 | 0 |
| submissionauth | 7 | 0 | 2 | 2 | 3 | 0 |
| proofledger | 7 | 7 | 0 | 0 | 0 | 0 |
| runnercontrol | 7 | 7 | 0 | 0 | 0 | 1 |
| secretstore | 7 | 7 | 0 | 0 | 0 | 4 |
| version | 6 | 2 | 2 | 1 | 1 | 1 |
| chitauth | 6 | 0 | 2 | 2 | 2 | 0 |
| distributionauth | 6 | 0 | 2 | 2 | 2 | 0 |
| paymentauth | 6 | 0 | 2 | 2 | 2 | 0 |
| retrievalauth | 6 | 0 | 2 | 2 | 2 | 0 |
| upgrade | 6 | 0 | 2 | 2 | 2 | 1 |
| deploy | 5 | 0 | 2 | 2 | 1 | 1 |
| wiring | 3 | 0 | 1 | 1 | 1 | 0 |
| github | 3 | 3 | 0 | 0 | 0 | 0 |
| gomodule | 2 | 2 | 0 | 0 | 0 | 3 |
| timeproof | 2 | 0 | 0 | 0 | 2 | 0 |
| awsidentity | 0 | 0 | 0 | 0 | 0 | 0 |
| capabilities | 0 | 0 | 0 | 0 | 0 | 1 |
| controlplanetest | 0 | 0 | 0 | 0 | 0 | 0 |
| gitrepo | 0 | 0 | 0 | 0 | 0 | 0 |
| gotoolchain | 0 | 0 | 0 | 0 | 0 | 0 |
| machineprobe | 0 | 0 | 0 | 0 | 0 | 0 |
| paypal | 0 | 0 | 0 | 0 | 0 | 0 |
| plunk | 0 | 0 | 0 | 0 | 0 | 0 |
| runworkspace | 0 | 0 | 0 | 0 | 0 | 0 |
| sourceclaim | 0 | 0 | 0 | 0 | 0 | 1 |
| sourceobservation | 0 | 0 | 0 | 0 | 0 | 0 |
| sourceproof | 0 | 0 | 0 | 0 | 0 | 0 |
| stripe | 0 | 0 | 0 | 0 | 0 | 0 |
| testserial | 0 | 0 | 0 | 0 | 0 | 0 |
| twilio | 0 | 0 | 0 | 0 | 0 | 0 |

## Remaining package queue

The September 10 queue reconciliation includes the locally reviewed releases
through Proofledger v2026.1.50. Since this ranking was captured, the Lineio,
Fuzzartifact, Lease, Retrieval, Manual, Receipt, Payment, Submission, Compass,
Googleidentity, Filelock, Submissionauth and Proofledger slices have landed.
Their package reports and committed source are review evidence; independent
runner acceptance remains a separate fact.

Continue in the user-approved September 10 order:

`retrievalauth`, `upgrade`, `deploy`, `wiring`, `github`,
`gomodule`, `timeproof`, `controlplanetest`, `gitrepo`, `machineprobe`, `paypal`,
`plunk`, `runworkspace`, `sourceclaim`, `sourceobservation`, `sourceproof`,
`stripe`, `testserial`, `twilio`.

Earlier skipped entry: `runprotocol` has scoped streaming changes but no
reconciled full-sweep report. Keep it open for reconciliation; the scoped
changes do not establish sweep closure. GCSobjects and GoToolchain retain the
separate follow-up status recorded above.

Before starting each slice, reconcile its individual report and commit history.
A generic before/after report from the bulk reservation audit is insufficient
proof of a full package upgrade. Explicitly requested follow-ups are tracked
separately from this queue, including the remaining GCSobjects sweep.

## Release checkpoint

Each package must have a complete public-boundary inventory; earned hostile
cases and applicable local layer triads; independent fuzz oracles for every
external ingress; typed refusal and exact effect/output proofs; and deliberate
mutation evidence for newly claimed ratchets. Capture original production before
editing. Benchmark comparisons use declared, checked workloads and retain both
CPU and memory profiles, binaries, commands and source bindings. Remaining known
surfaces keep the package open. Follow the entire local testing protocol.

The user approved the v2026.1.35 Distribution release. Each new slice retains
the same package-by-package standards and requires review before commit.

Then update Blink Kernel, Peachfuzz, Bug and Witness to that published version,
regenerate complete vendor trees where applicable, update actual call sites for
any contract change, and validate each consumer against the published module
with workspace replacement disabled. At the original usage snapshot, Blink declared v2026.1.18;
Peachfuzz, Bug and Witness declared v2026.1.5 and had vendor trees. Recheck their
actual versions before migration. Their shared
workspace points at local Primitive, so an ordinary workspace build alone cannot
prove published-version adoption. Preserve unrelated dirty consumer work.

Consumer migration remains pending.
This report preserves historical usage evidence without treating it as proof of
current published-version adoption.

Runnercontrol progress: v2026.1.51 fixed Go output metadata; v2026.1.52 closes the coverage-reader streaming slice. The package remains first in the queue. See [coverage streaming evidence](runnercontrol_coverage_streaming_20260910.md).

Runnercontrol v2026.1.53 admits interleaved Go build events without counting dependencies as test units. JSON streaming and the package sweep remain open; see [build-event evidence](runnercontrol_build_events_20260910.md).

Runnercontrol v2026.1.54 completes this review: fixed-window Go JSON, nominal
document ownership, JUnit framing, producer/accounting contradictions, snapshot
ownership, lifecycle sealing, strict errcheck and the public ingress fuzz
inventory. Its XML token and nominal-object memory contracts are explicit in
[the completion report](runnercontrol_completion_20260910.md). Earlier open-slice
notes above are historical. Independent acceptance and consumer migration remain
separate. Secretstore is now first in this queue.

Secretstore v2026.1.55: [local upgrade review](secretstore_upgrade_20260910.md). Provider SDK message materialization is explicit; independent acceptance is not claimed.

Version v2026.1.56: [local upgrade review](version_upgrade_20260910.md). Fixed nominal JSON-token memory; Chitauth is next.

Chitauth v2026.1.57: [local upgrade review](chitauth_upgrade_20260910.md). Distributionauth is next.

Distributionauth v2026.1.58: [local upgrade review](distributionauth_upgrade_20260910.md). Paymentauth is next. Composed JSON ownership and independent-acceptance limits remain explicit.

Paymentauth v2026.1.59: [local upgrade review](paymentauth_upgrade_20260910.md). Retrievalauth is next. Independent acceptance and nested JSON streaming ownership remain separate.
