# Primitive v2026.1.22

This user-reviewed checkpoint publishes the completed Contextstate and
Controlwire upgrades. Go 1.27.1 remains the toolchain. Consumers can select the
new module tag; their dependency pins are not changed by publishing it.

Contextstate preserves Go's exact context terminal sentinels and skips recovery
work on normal returns. Its exhaustive enum and hostile capability tables prove
single observation, panic containment and refusal of counterfeit terminal errors.
All eight final benchmark workloads allocate zero bytes per operation; profiles
show the normal-return recovery overhead removed. See the
[Contextstate review](../contextstate/upgrade_review.md) and its
[source-bound evidence](../contextstate/upgrade_evidence.json).

Controlwire enforces JSON admission ceilings, request-owned replay and HTTP byte
budgets, exact zero results on failed validation, and safe refusal of nil requests
and corrupt support counts. Nonce parsing uses Core's fixed-buffer canonical hex
decoder backed by Go's encoding/hex and now allocates zero bytes per text parse.
No network runtime, replay store or product policy was introduced.

Controlwire passed 5,255 leaf tests under race detection and eleven serial fuzz
campaigns totaling 2,699,088 executions. Ten before/after workload pairs retain
CPU and memory profiles and matching binaries. Timing samples, including the
small increases for replay and receive validation, are reported without claiming
a statistical trend. See the [Controlwire review](../controlwire/upgrade_review.md)
and its [source-bound evidence](../controlwire/upgrade_evidence.json).

Both packages completed their recorded scoped analyzers and native Darwin/arm64
tests. Linux/amd64 and Windows/amd64 checks compiled test binaries; they were not
native runtime tests. Full-module gates were not repeated for this checkpoint.
The release-coordinate check and review-file digest are recorded in
[release evidence](release_v2026.1.22_evidence.json). Large raw profiles, binaries
and command logs remain ignored locally; the reports are included in Git.

The user approved the supplied clean Grok review and explicitly authorized the
version bump, commit and push. Keygen is the next package in the usage-priority
queue. This release does not claim the remaining packages have completed their
hardening reviews.
