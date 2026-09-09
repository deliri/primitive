# Primitive v2026.1.32

Shutdown validates registration and run admission before retaining or consuming
work. Skipped and started steps preserve exact terminal and callback errors.
Signal observation captures the parent Done channel at construction, contains
panics on owned context calls and unlinking, and returns their original typed
causes. Close joins the observer and releases its subscription once, including
cleanup failures. ContextPanicError exposes bounded safe diagnostics and Unwrap.

Go context, os/signal and Temporal continue to own the mechanics. Plan uses its
existing mutex for registration and single use, with a fixed 64-step bound.
There is one observer goroutine and no replacement scheduler or cancellation
runtime. Callback termination remains cooperative. Invalid caller context methods
running in Go's own fallback goroutines remain outside the containment guarantee.

The reviewed final Linux race run has 259 passing events, no failures or skips,
and 96.4% statement coverage. Scoped vet, Staticcheck, Errcheck, Witness and
production complexity checks pass; macOS arm64 and Windows amd64 binaries compile.
Four follow-up behavioral mutations and six fuzz campaigns (7,674,450 executions)
verify the review corrections. Original proof remains recorded separately.

The user waived further Shutdown benchmarking. The already-completed original,
candidate and follow-up samples, CPU/memory profiles and matching binaries remain
outside Git. Shared-host timings are not statistical performance claims.
Full-module gates, native macOS/Windows execution and consumer updates were not run.

The user explicitly approved bump, commit, push and continuing to Currency.
See [the reviewed report](shutdown_upgrade_20260909.md),
[the current source evidence](shutdown_upgrade_20260909_review_followup.json),
and [release evidence](release_v2026.1.32_evidence.json).
