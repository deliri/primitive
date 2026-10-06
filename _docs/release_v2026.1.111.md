# Primitive v2026.1.111 — typed ambient environment execution

Parent: v2026.1.110, `2e4960dcd2d97b2c55016565d0960da3abf9158d`.

Hostfacts now executes a single current-process environment binding through
`SetAmbientEnvironment(context.Context, process.EnvironmentVariable)` and
`RemoveAmbientEnvironment(context.Context, process.EnvironmentName)`.
Process owns the existing name and value contracts. Hostfacts validates context
and intent before calling Go's native environment operations. Empty values remain
present; removal preserves native absence semantics. Callers own coordination,
permission and lifetime. No environment snapshot, restoration model, mutable
global dependency or compatibility path is introduced.

The existing capability catalog already assigns `os.Setenv` and `os.Unsetenv`
to Hostfacts. This supplies the missing executable boundary, enabling cooperating
products to remove those direct substrate calls.

Development accounting is retained under
`_docs/work/2026-10-06/ambient-environment/`. The absent API is the red state.
Complete Hostfacts/capabilities race suites passed after advancing the owned
export inventory. Semantic fuzzing ran for three seconds with two workers and
11,830 executions. An overlay removing context gates caused the refusal test to
fail on nil context, cancellation and canceled removal; production source retains
the gates. A final targeted race run passed after isolating mutation table cases.
Targeted vet and changed-function complexity checks are recorded separately.

These are local development results. Repository-wide Primitive gates, platform
coverage and independent acceptance are not claimed. Exact committed-source
verification is recorded separately from dirty development evidence.
