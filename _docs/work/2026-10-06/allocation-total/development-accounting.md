# Allocation total development proof

Production observation: Hostfacts reads runtime.MemStats.TotalAlloc without collecting. Witness needs cumulative allocation to test refusal before an encoded allocation; collected live heap cannot establish that budget.

First behavioral race run passed. Full Hostfacts race run failed only because the new public entry was inserted before an alphabetically earlier existing entry; inventory order corrected. This failed attempt remains a failure.

Actual production mutation: replace stats.TotalAlloc with stats.HeapAlloc. The first oracle, retaining its allocation fixture, survived this mutation. Strengthened the independent runtime adapter oracle by reclaiming the fixture and requiring cumulative allocation to remain monotonic. Repeated mutation then failed:

    TestGoAllocationTotalObservesCumulativeRuntimeBytes
    total after reclamation = 251496/<nil>, want >= 2346360
    FAIL github.com/deliri/primitive/v2026/hostfacts 0.255s

Command: go test -race -count=1 ./hostfacts -run '^TestGoAllocationTotalObservesCumulativeRuntimeBytes$'

Production mutation restored before the passing race run. This is development evidence, not independent acceptance or whole-repository verification.
