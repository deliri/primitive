# dsrv outbound capability review

Base: Primitive v2026.1.26, 8eedc412d3edf3b0490050407df1fe451ae34eef.

Adds a lazy outbound Tailscale SDK capability pinned to one address and port,
with bounded enrollment and an owned close point. Google metadata supplies
Tailscale federation identity. Product code supplies policy and coordinates.
No live enrollment has been performed. Destination checks are not authorization.

Adds an explicit service-account file identity source for a native worker.
Credential reads are bounded to 64 KiB through filestore; acquisition uses the
Google SDK through a bounded Primitive HTTP transport. Personal ADC is not a
fallback. No credential has been provisioned by this change.

Local development evidence on dsrv, Go 1.27.1, dirty source over the base above:
- adapter-green-126-03: go test -p=4 -count=1 -race -json -timeout=3m ./googleidentity ./tailnet; 241 passing events, no failed/skipped events.
- adapter-vet-126-02: go vet -p=4 ./googleidentity ./tailnet; exit 0.
- adapter-fuzz-126-01: 20-second service-account acquisition fuzz pass; 211 executions, local provider only.
- adapter-complexity-126-01: changed production functions at or below 10.
- credential byte-ceiling and pinned-destination one-fact mutations both fail the named tests; original source restored. Mutation hashes and commands are retained.

Raw commands, source hashes, output, prior failures and corrections are under
/work/evidence/d/anvil-dsrv on dsrv. These are local execution facts, not an
independent acceptance receipt. Tests do not prove live Tailscale enrollment,
Google credential issuance, App Engine resource use, or Anvil callbacks.
The SDK acquisition fixture exercises real signed-request construction against
a local provider; it does not claim that a local provider is Google.

No deployment, account credential, IAM role, firewall or paid compute changes.
