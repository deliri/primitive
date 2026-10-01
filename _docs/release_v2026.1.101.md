# Primitive v2026.1.101

Plunk now exposes typed resource routes for contacts and campaign create, read,
update, send/schedule, cancel, test and statistics operations. The SDK binds the
provider host, method and path, applies credentials, and executes one HTTP
attempt through Exchange. The calling Bridge owns protocol DTOs; Kernel and its
distros own audience, consent, content, schedules, durable claims and budgets.
No application state machine, background sender or retry loop was added here.

Resource IDs cannot introduce traversal, query, fragment or escaped delimiters.
Read operations use the download door; writes use the streaming round-trip door.
Existing v1 calls retain their original contract. DELETE uses single-attempt
semantics; keyed POST/PATCH preserve their caller-owned idempotency key.

Verification: 77 Plunk tests/subtests passed with the race detector on macOS and
Furnace, without skips. Semantic route fuzzing passed with a three-second budget.
Restoring the previous v1-only transport through a Go overlay caused the five
new real-HTTP resource scenarios to fail, while refusal scenarios still passed.
The overlay never modified the production checkout. Furnace go vet passed.
These are implementation proofs, not independent acceptance.

Retained proof: Cleanlift/_docs/work/2026-09-30/kernel-review/primitive-plunk
and Furnace /data/evidence/processed/primitive-plunk-101. Source archive SHA-256:
2fae124cac3c559b7322f98ea618accdb54bd28137ef22b017803637651637ac.
The archive covers runtime/test source before this release-only metadata bump.
