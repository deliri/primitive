# Primitive verifier slice: uncommitted review handoff

The Google service-account verifier now receives an admitted Exchange client.
The official Google SDK uses that client's transport behind Primitive's existing
bounded response and redirect controls. A typed RS256 header gate rejects other
algorithms, missing key identifiers, ambiguous JSON and oversized headers before
certificate acquisition. Product authorization remains outside this verifier.

This slice is left **uncommitted** for the user's other agent. Do not stage the
whole working tree: capabilities, sourceobservation, gotoolchain, Exchange socket
and response-buffer work belong to another concurrent effort.

## Files owned by this slice

- `exchange/official_sdk_client.go`
- `exchange/official_sdk_client_test.go`
- `googleidentity/verifier.go`
- `googleidentity/verifier_header.go`
- `googleidentity/data_flow_inventory.go`
- `googleidentity/identity_test.go` — adds the actual `Verify` method to the ingress inventory.
- `googleidentity/verifier_fixture_test.go`
- `googleidentity/verifier_hostile_test.go`
- `googleidentity/verifier_header_test.go`
- `googleidentity/verifier_provider_test.go`
- `googleidentity/verifier_mutation_test.go`
- `googleidentity/verifier_fuzz_test.go`
- `googleidentity/testdata/verifier_rsa.pem`
- `googleidentity/testdata/verifier_foreign_rsa.pem`

The two PEM files are public test-only signing keys. They have never represented
a Google account or production credential. Successful tests use real RSA
signatures and actual loopback TLS certificate responses through the public
`GoogleCloudVerifier.Verify` method and Google's SDK. They do not prove that
Google issued a live identity token.

## Behavioral evidence

The signed ingress table checks exact admitted identity facts, typed refusal,
zero proof on failure, certificate request counts, subject/email bounds, temporal
overflow, audience and issuer binding, foreign signing keys and unsupported
algorithms. A redundant unknown-algorithm row was removed because the unsigned
algorithm case already exercises that refusal contract.

Header tests sign the precise mutated header bytes. Valid headers originate in
the production typed marshaler. Duplicate and unknown members, wrong types,
null, truncation, trailing documents and below/at/above/extreme sizes reach the
real verifier. Refused headers cause zero certificate reads.

Certificate layer tests hold the signed document fixed and vary the actual TLS
response: admitted keys, empty response/key set, denial, redirect, truncated
declared length, invalid JSON and below/at/above/extreme response sizes. A separate
cancellation test waits for a real body read to start, cancels it and observes
both verifier and server-handler exit.

Mutation pairs first verify a baseline, alter one signed claim without renewing
its signature, require typed refusal and zero identity, replay the unchanged
baseline, then authenticate a freshly signed mutation. Exact identity facts and
issuer/subject principal derivation are checked across that sequence. Email
renaming preserves the principal; subject replacement changes it.

The Exchange adapter has its own real TLS positive, negative and empty-response
triad, exact byte-boundary assertions and invalid-construction checks. Replacing
the owned transport with `http.DefaultTransport` through a temporary Go overlay
made its TLS tests fail. The production source was never modified by that check.

Fuzzing calls the public verifier with mutated bearer values. Acceptance must
match an actual trusted signed document and its exact identity; a known valid
signed seed must also be accepted. Refusal must preserve the typed boundary
identity and return zero proof. Canonical identity serialization is checked for
closure. A second fuzz target checks the signing-algorithm wire boundary, and
the uint8 enum's complete domain is checked exhaustively.

## Validation and retained attempts

Local artifacts are at `/private/tmp/chunnel-primitive-verifier-evidence-final/`:

- `execution.json`: complete final race-test argv, directory, HEAD, dirty-tree
  facts, Go toolchain, selected environment, cache posture, exit, package states
  and emitted test/subtest terminal events.
- `sources-before.json` and `sources-after.json`: dependency-source digests;
  unchanged across the final execution.
- `race.jsonl`, `race.stderr`: complete final execution output.
- `source/`, `tracked-changes.patch`, `owned-files.json`: reviewable source snapshot
  and ownership list for this slice.
- `history/`: earlier red/green attempts, fuzz runs, lint output and mutation files.
- `artifact-index.json`: byte lengths and SHA-256 digests of retained files.

Final package execution:

```text
go test -count=1 -race -json ./googleidentity ./exchange
```

Exit 0 on Go 1.27.1 darwin/arm64, Primitive HEAD
`b13df4c1878c69106dda26b005ee440f4f30c40f` with uncommitted changes.
Both packages passed; 1,074 emitted test/subtest terminal pass events, no emitted
fail or skip events. These event counts are not a count of independent proofs.

Additional checks passed:

```text
go vet ./googleidentity ./exchange
staticcheck ./googleidentity ./exchange
witness-lint ./googleidentity ./exchange
gocyclo -over 10 googleidentity/verifier.go googleidentity/verifier_header.go exchange/official_sdk_client.go
go test -count=1 -json ./cmd/api -run '^TestAnvil'  # Blink Kernel directory
```

Fuzz executions:

```text
go test -count=1 -json ./googleidentity -run '^$' -fuzz '^FuzzGoogleCloudVerifierSignedSemanticClosure$' -fuzztime=30s -fuzzminimizetime=1s -parallel=1
go test -count=1 -json ./googleidentity -run '^$' -fuzz '^FuzzGoogleCloudSigningAlgorithmSemanticClosure$' -fuzztime=10s -parallel=1 -fuzzminimizetime=1s
```

Both exited 0: 6,085 verifier executions and 42,643 algorithm executions. An
earlier 30-second verifier fuzz run reported only ten executions; its output is
retained and does not substantiate a meaningful search. A diagnostic 100-execution
run completed before bounded minimization was used for the subsequent search.

Retained failures include the original ES256/missing-key certificate-read red
state; a `contractError(nil)` implementation mistake; a redirect fixture whose
HTML body hit JSON refusal before redirect handling; eleven doctrine failure-
message findings; and the deliberate default-transport mutation. The redirect
fixture now returns a bodyless 302 so it actually reaches redirect refusal.
Earlier runs did not retain complete source snapshots and execution metadata;
their terminal output is supporting local evidence with that limitation.

## Acceptance and remaining product work

This is local development evidence, not an independent acceptance receipt. The
final execution retains emitted events and selected package scope but lacks an
independently discovered per-test denominator. The other agent must review,
commit and obtain independent execution/verification on that exact commit.
No benchmark behavior changed and no benchmark acceptance claim is made.

The producer/classifier transition matrix belongs to the downstream product
handoff; these Primitive tests exercise authentication and transport admission.
They do not close Anvil or Blink's classification, durable acceptance, ledger,
manifest, delivery or lifecycle surfaces.

Blink's two verifier constructors now supply an Exchange client. Its real
successful identity-to-peer middleware proof still needs completion. Anvil still
needs authenticated Blink requests instead of private-IP identity, production
daemon/lease wiring, delivery accounting and terminal lifecycle evidence. Live
Google service-account impersonation was unavailable to this session's account;
no IAM grants or other security configuration were changed.
