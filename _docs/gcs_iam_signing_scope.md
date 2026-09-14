# GCS capability signing OAuth scope

`NewGCSCapabilityIssuer` uses the same authenticated HTTP transport as the
Storage client. That transport previously requested only Storage full-control
scope, overriding the IAM SDK's normal scope selection. A principal with
`iam.serviceAccounts.signBlob` permission therefore still received a scope
refusal when issuing upload capabilities.

The shared transport now retains Storage full-control scope and includes the
official IAM Credentials SDK's `CloudPlatformScope` constant. IAM permissions
still govern the operation; no credential, retry policy, or signing mechanism
is replaced. The SDK owns credential discovery and token exchange.

Reference: [Google signBlob authorization scopes](https://docs.cloud.google.com/iam/docs/reference/credentials/rest/v1/projects.serviceAccounts/signBlob).

`TestGCSCapabilityAuthenticationLayerTriadPreservesSigningScope` uses the real
credential-file constructor, authenticated transport, OAuth JWT exchange and
official signing SDK. The local token endpoint observes the requested scopes;
the local signing endpoint refuses a storage-only token. Positive issuance,
provider refusal with zero capability, and canceled ingress with zero token and
signing calls are pinned. Every subtest owns its credential tempdir and HTTP
server. The credential key is deterministic and synthetic. Endpoint substitution
is a test seam; the returned signature is synthetic and is never spent on GCS.

Base revision: `5c759e59deefc970dd2619554f513b3ef7283bd0`.
Local execution receipts under `/private/tmp/anvil-execution-<id>` retain exact
argv, dirty source digests, toolchain, output hashes/bytes and process status:

- `BLoCUJ`: behavioral red before the production fix. Signing returned typed
  destination failure with HTTP 403; both active rows detected the absent IAM
  scope. Canceled ingress passed.
- `IOBtbx`: fixed triad under `-race -count=1`, four passing events, no skips.
- `vCxhCC`: complete `go test -json -count=1 ./gcsobjects`, 801 passing events,
  no failures, three explicit live-provider skips (authenticated deletion with
  retention, authenticated lifecycle/deletion, private download capability).
- `zzQjpg`, `4aQThz`, `2R4pOk`: vet, staticcheck and witness-lint respectively,
  all exit zero for `./gcsobjects`.
- `BJVJkw`: credential-file ingress fuzz once with `-fuzztime=3s`,
  `-fuzzminimizetime=3s`, and one worker; exit zero.

These are local facts, not independent acceptance. ADC and live IAM signing
remain deployment proof surfaces; the credential-file test does not claim to
exercise the metadata server. No new parser or public payload was introduced.
