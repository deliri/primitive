# Primitive streaming audit — every package

User requirement, 2026-09-09: Primitive must process reads and writes through fixed memory windows, regardless of whether the source contains 1 TB or 100 TB. Buffer budgets control working memory and backpressure; they must not become arbitrary file, line, object, request, response, or total-transfer acceptance quotas. Validation stays with the owning contract. Native I/O errors and cancellation remain observable.

This requirement applies to every package, including packages whose earlier sweeps are already published. Earlier completion is historical and does not exempt a package. Implementation remains package by package, with before/after profiles and hostile streaming proof. Lineio now emits fixed-buffer fragments; its review follow-up is recorded. The current uncommitted slice removes transfer quotas from the reviewed Filestore, Exchange, and GitHub streaming doors. Focused verification and profiles are retained outside Git; downstream product migration remains outstanding. Witness source migration is now user-owned, per the latest instruction; the installed witness-lint remains a required Primitive check.

The inventory below is a navigation aid, not semantic verification. It includes every module package discovered by go list and scans production source plus ignored platform variants for likely limits/materialization. Zero matches does not mean compliant. Each package still requires read/write path review. Provider-specified and native representability constraints must be distinguished from Primitive-invented transfer quotas; this inventory has not verified the provider contracts.

Audit exit per package: no hidden transfer/line quota; reader and writer paths conserve exact bytes; fixed memory windows continue across larger inputs; backpressure and caller-owned cancellation remain direct; errors preserve identity; finite test workloads demonstrate continuation without claiming a full terabyte transfer was executed; typed API changes update actual callers without compatibility shims. APIs that require complete materialized values must expose that ownership rather than pretend to be fixed-memory streaming.

63 packages inventoried. All remain subject to this new audit.

| Package directory | Candidate source lines | New streaming audit |
|---|---:|---|
| attest | 36 | requires_review |
| awsidentity | 11 | requires_review |
| capabilities | 5 | requires_review |
| chit | 26 | requires_review |
| chitauth | 7 | requires_review |
| compass | 7 | requires_review |
| contextstate | 0 | requires_review |
| controlplane | 60 | requires_review |
| controlplanetest | 0 | requires_review |
| controlwire | 16 | requires_review |
| core | 100 | requires_review |
| currency | 15 | requires_review |
| deploy | 0 | requires_review |
| distribution | 42 | requires_review |
| distributionauth | 19 | requires_review |
| exchange | 86 | streaming_doors_reviewed |
| filelock | 0 | requires_review |
| filestore | 13 | streaming_doors_reviewed |
| fuzzartifact | 2 | streaming_doors_reviewed |
| gcsobjects | 47 | requires_review |
| github | 39 | streaming_doors_reviewed |
| gitrepo | 0 | requires_review |
| gomodule | 0 | requires_review |
| googleidentity | 33 | requires_review |
| gotoolchain | 2 | requires_review |
| hostfacts | 27 | requires_review |
| id | 6 | requires_review |
| keygen | 6 | requires_review |
| lease | 88 | requires_review |
| lineio | 0 | ready_for_review |
| machineprobe | 5 | requires_review |
| manual | 3 | requires_review |
| objectstore | 56 | requires_review |
| payment | 21 | requires_review |
| paymentauth | 7 | requires_review |
| paypal | 27 | requires_review |
| plunk | 4 | requires_review |
| process | 18 | requires_review |
| proofledger | 22 | requires_review |
| receipt | 33 | requires_review |
| release | 57 | requires_review |
| retrieval | 14 | requires_review |
| retrievalauth | 7 | requires_review |
| runnercontrol | 100 | reviewed; explicit nominal/XML-token ownership in [completion report](runnercontrol_completion_20260910.md) |
| runprotocol | 14 | requires_review |
| runworkspace | 12 | requires_review |
| secretstore | 20 | reviewed; explicit SDK message ownership in [upgrade report](secretstore_upgrade_20260910.md) |
| shutdown | 0 | requires_review |
| sourceclaim | 0 | requires_review |
| sourceobservation | 0 | requires_review |
| sourceproof | 0 | requires_review |
| stripe | 5 | requires_review |
| submission | 32 | requires_review |
| submissionauth | 17 | requires_review |
| tailnet | 7 | requires_review |
| tailnetconfig | 6 | requires_review |
| temporal | 23 | requires_review |
| testserial | 0 | requires_review |
| timeproof | 34 | requires_review |
| twilio | 3 | requires_review |
| upgrade | 16 | requires_review |
| version | 0 | requires_review |
| wiring | 0 | requires_review |

Lineio review evidence: [review follow-up](lineio_review_20260909.md). Its Go source is verified; user review is pending. The remaining package-wide audits are not complete; the scoped review below does not establish whole-package compliance.

Filestore caller migration is not completion of its consumers. In particular, Google authentication SDKs currently require materialized credential JSON; Googleidentity and GCSobjects retain that explicit whole-document ownership pending their provider-specific streaming review. Upgrade persistence and Go-source discovery also require their own decoder/aggregation review. Removed Filestore quotas are not silently replaced elsewhere.

Streaming slice review, 2026-09-10: `/tmp/streaming_slice_review_20260909_findings.md` on Furnace reviewed the dirty tree based on `5d0811a74fef4a64ddedfeaff0f1fd7d4c6bff16` and found no production bug on the claimed GitHub-to-Filestore path. `streaming_doors_reviewed` records that limited production review, not a released revision or a whole-package audit. The reviewer reproduced selected Linux builds, not the author's test, fuzz, race, or profile runs. Candidate-line counts above remain the original inventory, not a fresh scan.

Preserving a previous file requires caller composition: stage, stream, abandon on producer error, otherwise finish and commit. GitHub writes directly to the supplied destination. Its metadata JSON operations still materialize complete responses. Archive redirect-body drains rely on the caller context; tree visitors provide synchronous backpressure and must return. Go's `io.Pipe` is unbuffered. Product consumers, including Blink Kernel's old GitHub file API, still require migration before upgrading this slice.
