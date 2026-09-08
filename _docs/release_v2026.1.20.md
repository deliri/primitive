# Primitive v2026.1.20

The user reviewed and approved the Temporal and Hostfacts batch on September 8,
2026, including the subsequent bug-report fixes. The sole version declaration
remains `compass/config.json`; the Go requirement remains 1.27.1.

Temporal preserves JSON error identity and receivers on refusal, closes interval
overflow admission, and improves standard-library-backed value projections.
Unset ticker access now panics with the typed Temporal contract error instead
of returning a nil channel. Valid access and stopping retain Go's tick channel.
See the [Temporal review](../temporal/upgrade_review.md).

Hostfacts corrects hybrid cgroup memory ownership and missing-directory handling,
preserves cgroup failure diagnoses, strengthens admission boundaries, and keeps
caller contract errors distinct from failed or unsupported native observations.
Its bounded OOM scan uses Go's byte-search implementation. HTTP, filesystem,
time and native observation retain their existing package/stdlib owners.
Windows physical-memory observation remains explicitly unsupported.
See the [Hostfacts review](../hostfacts/upgrade_review.md) and
[review follow-up](../hostfacts/review_followup.md).

The requested module analyzers and all 61 package test suites passed before
release approval. Later narrowly scoped changes received affected-package
analyzer, race and cross-compilation refreshes, detailed in the follow-up.
Native runtime evidence is Darwin/arm64; Linux/amd64 and Windows/amd64 are
compile checks. Three credentialed GCS tests skipped outside these packages.

Reports retain before/after measurements with CPU and memory profile bindings,
exact binaries, command/source hashes, unsuccessful attempts and uncertainty.
Raw profiles, binaries and large logs stay ignored locally. Reports and source
accompany this release. The version-only change receives Compass/Version checks;
the previously completed full gates are not rerun for a coordinate bump.

The next review batch is Process, Core and Contextstate, one package at a time.
Consumer module/vendor upgrades remain a separate rollout; publishing this tag
does not itself change those projects' pinned dependencies.
