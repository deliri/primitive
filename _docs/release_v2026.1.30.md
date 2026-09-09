# Primitive v2026.1.30

Manual topic names admit dotted command identities. Each dot separates a
nonempty segment of lowercase letters, digits, and single interior hyphens.
The existing total byte limit remains authoritative. Empty segments, boundary
hyphens, uppercase text, invalid UTF-8, and oversized names remain invalid.
Admission preserves the exact spelling; no aliases or normalization are added.

This enables cooperating tools to document their compiler-owned command names
through Primitive's existing bounded book, text, and JSON contracts. Product
guidance remains with the consuming tool.

The reviewed change passed manual-package race tests, vet, Staticcheck,
errcheck, and a 10-second, one-worker grammar fuzz campaign with 247,722
executions on macOS and Go 1.27.1. Seventeen explicit grammar boundary cases
and the independent fuzz oracle protect admission and typed refusal.
Full-module gates and live external effects were not part of this slice.
Review artifacts are retained in `/private/tmp/manual-upgrade-evidence`;
release checks and final source coordinates are recorded separately in
`/private/tmp/manual-release-20260909`.

The user explicitly approved the coordinated version bump, commit, push,
SwissKnife dependency upgrade, and Mac installation after review.
