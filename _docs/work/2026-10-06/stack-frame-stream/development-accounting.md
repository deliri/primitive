Streaming current-goroutine stack proof

Production delegates to runtime.Callers and runtime.CallersFrames using one fixed 32-counter window. It revisits the real Go stack for successive windows, so traversal work grows with depth while retained memory stays constant. It creates no stack inventory, no goroutine and no simulated runtime state. Runtime symbol/source strings remain Go observations.

Independent provider oracle: runtime.Callers into a known sufficient 512-counter test fixture and runtime.CallersFrames. Recursive depths 0, 1, 31, 32, 33, 64, 127 and 256 prove boundary crossings and exact recursive-frame count. Lifetime tests prove consumer stop, cancellation after one observation, nil context identity, and validation of unavailable source coordinates.

Actual one-window production mutation returned after the first 32 counters. Six depth cases failed with 31 recursive observations instead of the complete independent counts. Production was restored before passing tests. Raw failure is window-mutation.stdout.

Development go test -race -count=1 ./hostfacts ./capabilities passed: Hostfacts 41.712s; capabilities 2.340s. Nil-context assertion was strengthened afterward; committed scoped proof follows separately.

Development go test -count=1 ./hostfacts -run '^$' -fuzz '^FuzzCurrentGoStackFramesMatchesRuntime$' -fuzztime=3s passed: 8 baseline seeds, 69319 executions, 3 new interesting inputs, 10 workers, package 3.249s. This is filtered development proof, not whole-repository verification or independent acceptance.
