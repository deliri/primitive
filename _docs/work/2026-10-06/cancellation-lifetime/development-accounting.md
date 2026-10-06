Primitive cancellation lifetime development proof

WithCancellation delegates propagation and first-cause ownership to Go context.WithCancelCause. No timers, goroutines or cause registry are introduced.

Restored production: go test -race -count=1 ./temporal passed (1.238s). This is development proof before the source checkpoint.

Actual production mutation substituted context.Background() for the supplied parent. go test -race -count=1 ./temporal -run '^TestCancellationLifetimeUsesGoFirstCauseAndParentPropagation$' failed: parent cancellation did not synchronously propagate. Mutated production was restored before the passing full Temporal run.

Independent acceptance remains with the user. This proof covers Temporal, not the whole Primitive repository.
