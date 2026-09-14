# Empty report acknowledgment

A committed empty report can preserve a project summary at revision zero. The
acknowledgment now permits that value while continuing to require a positive
report sequence, bounded revision, valid identity, future schedule and signature.
This does not classify a queued report as aggregated.

Retained local evidence under /private/tmp/peachfuzz-state-readonly-evidence/:
- primitive-empty-report-ack-red-20260914: zero revision failed on v2026.1.93 production.
- primitive-empty-report-ack-green-20260914: 259 test events passed.
- primitive-empty-report-ack-fuzz-20260914: semantic signed closure passed, 2s fuzz + 1s minimization.
- primitive-empty-report-ack-vet-20260914: exit zero.

Receipts bind base commit, dirty source snapshots, exact command, toolchain,
stdout/stderr digests and exit. These local runs do not provide independent
acceptance, production Firestore verification or performance claims.
