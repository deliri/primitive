# Native streaming Twilio webhook verification

`WebhookReceiveRequest` requires the caller's `ScratchParent`. Exchange owns
HTTP ingress and body closure; Filestore owns the temporary namespace, native
files, replay lifetime and cleanup. The receiver hashes borrowed credential
bytes without converting the secret to a Go string. Closing the shared custody
refuses copied receivers before reading ingress.

Form bodies are staged once and indexed as opaque native source coordinates.
The existing Filestore external merge sort orders decoded names; decoded names
and values stream into the protocol's HMAC-SHA1 hashes. Duplicate decoded names
remain a refusal because the Go SDK authenticates only their first value.
Percent escapes, plus spelling, arbitrary field extent and physical newlines
do not require a field map or retained form body.

JSON callbacks hash the exact raw body and bind the provider's `bodySHA256`
query field. Query comparison preserves each decoded name's value order while
allowing equivalent escaping and independent name order. Repeated scans retain
only coordinates. Their work is quadratic in query field count plus comparison
cost; this is not a linear-time claim. Native Go URL host spelling preserves
IPv6 brackets for both accepted port spellings.

Fixed transfer and sorting buffers impose no webhook body, field extent or
field cardinality quota. Scratch storage grows with the native input and index;
retained working memory does not retain their complete contents. Destination
effects occur only after authentication and remain subject to native write and
cleanup errors.

The maintained Twilio SDK is an independent test oracle. Native tests include
64 form cases, 64 raw JSON cases, 64 IPv6 cases, shared-close custody and
complete-byte native read failures. Payloads exceed six MiB; form cardinality
crosses the existing 4096-record merge run. Semantic fuzzers exercise full input
without clamping or skipping its extent. Producer failures, cancellation,
malformed tuples and panic exercise the shared Filestore index scope.

Development proof lives under
`/data/evidence/raw/primitive/2026-10-11-twilio-stream` on Furnace. Earlier
inventory failures and a benchmark fixture's second close are retained. The
4 KiB, 8 MiB and 64 MiB native-file benchmark measures allocated bytes per
request, excluding fixture allocation; it does not measure a heap or RSS bound.
Isolated production mutations bypass signature, raw digest or query checks,
retain secret custody, hash the wrong form span, and lose IPv6 brackets; actual
native tests fail and pass after restoration.

Remaining surfaces include provider HTTP SDK imports and existing endpoint and
credential contracts, PayPal webhook aggregation, Exchange whole-value APIs,
Timeproof observation binding and consumer dependency upgrades. This slice is
not completion of those surfaces or independent acceptance.

Protocol references:

- https://www.twilio.com/docs/usage/security
- https://www.twilio.com/docs/usage/webhooks/webhooks-security
