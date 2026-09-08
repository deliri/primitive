# Primitive v2026.1.24

This user-reviewed checkpoint publishes the Release package upgrade on Go 1.27.1.

Dependency admission now preserves exact canonical checksums, explicit collection
presence, repository root/commit binding, and Go module error facts. Sealed indexed
access no longer revalidates the entire collection on each access. Signing-seed
admission bounds input and preserves or consumes custody at the owning boundary.

Executable inspection uses Go's parsers and bounded streaming hashes, checks path
standing before and after reading, and preserves cooperative cancellation.
VerifyBuildTools resolves executable links through Filestore and uses the resolved
path for inspection, execution and the returned proof. Returned paths may therefore
differ from supplied aliases. Explicit links outside the alias directory are
resolved too; no caller-supplied confinement root exists in this request.

The final Release race run passed 1,193 test/subtest events with zero failures or
skips and 84.8% statement coverage. Scoped analyzers passed. Linux amd64/arm64 and
Windows amd64 compiled test binaries only. The eight retained timed fuzz campaigns
precede the final build-tool review fix; their decoder and artifact paths did not
change. Full-module gates were not repeated.

Twelve before/after benchmark pairs retain requested 30-second workloads, explicit
CPU/memory profiles, source snapshots and matching binaries. The affected compiler
inspection measurement was refreshed after review fixes. Timing samples are mixed
and do not establish statistical trends. See the [review](../release/upgrade_review.md)
and [package evidence](../release/upgrade_evidence.json).

Fresh Compass and Version coordinate tests are recorded in the
[publication evidence](release_v2026.1.24_evidence.json). Large raw profiles,
binaries and logs remain ignored locally; their reports are included in Git.
The user explicitly approved bump, commit and push. Objectstore is next;
consumer dependency pins are unchanged by this publication.
