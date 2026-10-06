# Reviewed text symbol ownership

Primitive classifies each reviewed package-level function in Go 1.27.1's
`strings`, `bytes`, and `strconv` packages. Callback-taking functions remain
contextual. An unknown selector, foreign package, or unlisted receiver remains
unresolved. This adds catalog knowledge; it does not claim transitive product
call coverage or complete Hammer source admission.

The initial production regression failed on the missing pure and callback
classifications. The actual compiler export and parameter oracle additionally
found missing `strings.Lines`, `bytes.Lines`, and `bytes.CutLast` contracts;
the reviewed standard-library implementations were added.

`go test -race -count=1 ./capabilities` passed after implementation. The complete
retained development run is `go test -json -race -count=1 ./capabilities ./compass`,
Go 1.27.1, darwin/arm64. Its raw `development-race.jsonl` contains 92,128 events,
22,268,967 bytes, both package passes, and no fail or skip events. Count 1 disables
test-result caching; compiler caches remain enabled. This run preceded the source
checkpoint and is development evidence rather than proof of a committed revision.

`go test -count=1 ./capabilities -run '^$' -fuzz '^FuzzTextSymbolNamespaceAndCallbacks$' -fuzztime=3s`
passed with 206,956 executions, 20 seeds, 40 new interesting inputs, and 10 workers
in 3.266 seconds. The independent namespace oracle tests pure/callback transitions,
foreign packages, receiver changes, and invented or invalid selector suffixes.

The actual production mutation moved every text callback selector into the pure
classification and removed its contextual classification. Both hostile boundaries
and the actual compiler parameter oracle rejected it. Full mutation output is
retained in `callback-mutation.stdout`. The production source was restored before
the passing full capability run. No mutation remains active.

Independent review and acceptance remain the user's responsibility.
