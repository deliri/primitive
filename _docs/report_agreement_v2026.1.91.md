# Report agreement v2026.1.91

The permit package now carries shared signed report, initial permission,
acknowledgment and refreshed authorization structures. ReportHead checks exact
replay or the immediate successor against a caller-supplied committed digest.
It does not authenticate a sender or persist anything. Callers verify signatures
and atomically commit their summary and checkpoint.

ReportSchedule calculates recurring half-open admission windows, clips jitter
at close and jumps over missed periods arithmetically. Products supply cadence,
initial phase, authorization and expiry. QuietPeriods contains at most 32 sorted
UTC exclusions per permission, separately selected for reports and object uploads.
This bounds one permission document; it does not bound captured evidence.
Blackout windows reduce concurrent eligibility; the transaction preserves totals.
Neither scheduling nor hashing promises exclusive slots across devices.

Authorization refresh contains no second schedule. ReportPermissionResponse
requires exactly one signed initial permission or committed acknowledgment.
Products own refresh precedence and durable cursor advancement. Primitive owns
strict decoding, domain/scope/signature binding and the timing arithmetic.

## Evidence and remaining work

The adjacent JSON retains every captured local attempt, commands, source revision,
dirty fact and output digests. Full receipts and source snapshots remain at the
listed local paths. These are local development facts, not independent acceptance.
The deliberate mutation changing the exclusive-close comparison from >= to >
failed the time-boundary test and was discarded. Earlier structural and lint
failures remain in the attempt sequence; later success does not erase them.

The permit suite passed 205 test events, with zero failures or skips, normally
and with the race detector. Focused vet, staticcheck and doctrine lint were run.
Semantic fuzz targets exercise schedule arithmetic, signed reports/permissions/
acks, refreshed authorization, quiet-period decoding and project/domain decoders.
Fuzz budgets are two seconds plus one second of minimization per target.

The broad core run exposed an existing TestRealWorldEffectOwnershipMatchesLandedProduction
inventory mismatch: committed gcsobjects/client.go imports net/http, while the
committed HTTP-owner list omits PackageGCSObjects. This release does not alter
those files. New core error identities, public documentation and JSON validation
witnesses were corrected and their targeted guards passed. The full repository
gate is not claimed clean.

API policy/checkpoint and CLI scheduling/retention integration, Firestore atomic
transaction proof, committed consumer builds, live uploads and independent
acceptance remain separate required work. No deployed behavior is claimed here.
