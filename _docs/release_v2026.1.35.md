# Primitive v2026.1.35

Distribution binds completion evidence to every granted upload capability;
Deploy preserves that identity and validates it in each receipt. Typed-nil
readers and writers fail at admission. Request commitments accept only the
three request payload types and reject missing domain tokens before hashing.
Upgrade stage projection validates root identity through Filestore.

The reviewed follow-up restores fail-closed token-table validation, rejects
empty parser input even if a future table entry is missing, and preserves
failed-upload attempt identity without allowing completion evidence.

Linux race checks passed: Distribution 1,491 passing events, 90.1% coverage;
Deploy 61 passing events, 87.9% coverage; no failures or skips. Scoped go fix,
Vet, Staticcheck, Errcheck, Witness and complexity checks pass. macOS arm64 and
Windows amd64 compile checks pass; no native execution is claimed.

Twelve final benchmark workloads retain matching CPU/memory profiles and
binaries outside Git. Three focused final fuzz campaigns passed 1,868,962
executions. The earlier seventeen-target campaign remains historical.
Shared-host samples and the fast parser iteration ceiling are documented.

The user explicitly approved bump, commit, push and the next package. Reviewed
Go sources remain unchanged. Only release coordinate tests are repeated here.
Full-module gates and consumer upgrades were not run. Lineio is next.

See [review fixes](distribution_review_20260909.md),
[review evidence](distribution_review_20260909_evidence.json), and
[release evidence](release_v2026.1.35_evidence.json).
