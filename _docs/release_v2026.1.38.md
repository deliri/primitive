# Primitive v2026.1.38

Author: Ase Deliri

Exchange now exposes SendTo and SendNoBodyTo with a typed, per-attempt
ResponseDestination and ResponseDelivery. These operations stream success and
unexpected-status response bodies through the existing HTTP, redirect, retry,
and Retry-After engine. Product code owns sink retention and admission policy.
Primitive does not impose a new transfer ceiling.

ResponseDelivery distinguishes acknowledged bytes from a body that reached EOF.
Status, native read/write/close failures, and cancellation retain their
identities. Each retry obtains a new caller-owned destination. Existing
whole-value operations use the same execution engine.

Validation on Go 1.27.1 linux/amd64, GOWORK=off:
- go build ./...
- go vet ./exchange
- go test ./exchange -count=1 -timeout=3m
- go test ./exchange -run '^$' -fuzz '^FuzzResponseDestinationPreservesDelivery$' -fuzztime=20s -parallel=4
- gocyclo -over 10 exchange/client.go exchange/response_destination.go
- git diff --check

The new delivery matrix covers both request forms, success/error statuses,
five transfer-window boundaries, and independent read, close, cancellation,
and sink-refusal outcomes. The fuzz run completed 280,045 executions.
This release does not claim a full repository test sweep or independent acceptance.
