# Primitive upgrade priority — 2026-09-07

Current approved release checkpoint: **v2026.1.22**, September 8, 2026.
The user reviewed and approved Contextstate and Controlwire, including the
version bump, commit and push. See [release notes](release_v2026.1.22.md).
The preceding GoToolchain/Filestore/Core integration was published in v2026.1.21.
Exchange/Filestore were published in
v2026.1.19 and Temporal/Hostfacts in v2026.1.20.

Reviewed checkpoint: **Contextstate**. The resumed slice closes its test and benchmark
review and makes a profile-informed recovery optimization. See
[Contextstate's review](../contextstate/upgrade_review.md). **Controlwire is reviewed and approved.** Its production, table tests, fuzz
oracles and profiled benchmarks are covered in [the review](../controlwire/upgrade_review.md).
**Keygen is next.**

The initial execution-value priority was Exchange → Filestore → Temporal →
Hostfacts → Process. Process and Core improvements are now checkpointed in
v2026.1.21; their earlier package reports retain the historical pre-release
wording. The new compiler integration can receive further upgrades in a later
release, as the user requested. Necessary shared-owner fixes are still proved at
their owning boundary when encountered.

Exchange covers Blink HTTP ingress/egress and the three tools' control-plane and
commercial clients. Filestore covers rooted reads/writes/walks and durable output
in all four consumers. Temporal underpins values, budgets and cancellation.
Hostfacts supplies ambient host facts and resource observations, including Blink's
real cgroup memory-limit path. Process supplies compiler, probe, replay and runner
execution. All five have production importers in all four projects.

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
| fuzzfinder | 15 | 0 | 9 | 0 | 6 | 0 |
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

## Queue after the first release batch

Order by descending direct production import breadth, then consumer breadth,
then package name. Recheck the scan at each release because usage may change.
Previously upgraded packages still require closure of known review gaps; a prior
commit is not a waiver. Attest's inventory binding gap remains recorded for its own follow-up;
the user explicitly approved the scoped Temporal/Hostfacts release. AWS identity and Capabilities retain
local evidence, with the promised manual compliance re-review still pending.
Chit's original baseline is retained; its upgrade is paused for this priority.

`keygen`, `release`, `objectstore`, `chit`, `attest`, `controlplane`, `gcsobjects`, `shutdown`, `currency`, `id`, `distribution`, `lineio`, `fuzzfinder`, `runprotocol`, `lease`, `retrieval`, `manual`, `receipt`, `payment`, `submission`, `compass`, `googleidentity`, `filelock`, `submissionauth`, `proofledger`, `runnercontrol`, `secretstore`, `version`, `chitauth`, `distributionauth`, `paymentauth`, `retrievalauth`, `upgrade`, `deploy`, `wiring`, `github`, `gomodule`, `timeproof`, `awsidentity`, `capabilities`, `controlplanetest`, `gitrepo`, `gotoolchain`, `machineprobe`, `paypal`, `plunk`, `runworkspace`, `sourceclaim`, `sourceobservation`, `sourceproof`, `stripe`, `testserial`, `twilio`.

## Release checkpoint

Each package must have a complete public-boundary inventory; earned hostile
cases and applicable local layer triads; independent fuzz oracles for every
external ingress; typed refusal and exact effect/output proofs; and deliberate
mutation evidence for newly claimed ratchets. Capture original production before
editing. Benchmark comparisons use declared, checked workloads and retain both
CPU and memory profiles, binaries, commands and source bindings. Remaining known
surfaces keep the package open. Follow the entire local testing protocol.

The user approved v2026.1.22 on September 8, 2026. Contextstate and Controlwire
are reviewed; Keygen is the next slice. Each new slice retains the
same package-by-package review standards and requires review before commit.

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
