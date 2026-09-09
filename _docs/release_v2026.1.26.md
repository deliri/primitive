# Primitive v2026.1.26

This reviewed release repairs the four packages that failed the first full Linux
race run after moving execution to the server. GCS reads now prove actual EOF
beyond the expected extent, preserve source and destination errors, and never
send the extra probe byte to the destination. Copying, transport and hashing
continue through Go and the existing Primitive capabilities.

Deploy and Distribution fixtures include the required Release selector collection.
GCS tests use correct empty digests and test-owned credentials with a local token
endpoint. Runnercontrol cancellation tests use an owned supervisor fixture that
accepts supervisor arguments and produces output deterministically.

The Linux race run passed all 61 packages, with four explicit live-provider or
platform-specific skips. Scoped vet, staticcheck and errcheck passed; GCS
complexity and Witness passed. The new public read fuzz campaign completed 7,550
executions without failure. Review f0073b5f found zero bugs; its comment and import
cleanup findings are resolved. Subsequent edits changed comments and import
grouping only. Full-module analyzer gates and macOS/Windows execution were not
repeated for this slice.

The identical-harness final benchmark pair retains CPU/memory profiles and matching
binaries. The 1 KiB read measured 75,714 -> 42,225 B/op; the 1 MiB read measured
73,247 -> 73,337 B/op and 502 -> 504 allocations. Timing samples are not a proven
speedup because the server had other work running.

See the [review and execution notes](linux_read_regressions_20260909.md),
[execution manifest](linux_read_regressions_20260909_evidence.json),
[review follow-up](linux_read_regressions_20260909_review_followup.json), and
[release evidence](release_v2026.1.26_evidence.json).

The user explicitly approved the version bump, commit and push. Raw profiles,
binaries, logs and source snapshots stay outside Git in the server evidence
directory. Consumer dependency pins are unchanged by this release.
