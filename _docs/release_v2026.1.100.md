# Primitive v2026.1.100

Adds `passwordhash`, a caller-configured Argon2id execution capability. Native
work and constant-time verification use Go's maintained crypto implementation.
Explicit resource limits and shared, non-queuing admission bound simultaneous
work. Cancellation preserves the caller's context error and never publishes a
cancelled result. Kernel and applications continue to own credential format,
pepper management, enrollment policy and stored-state transitions.

Tests cover native and caller boundaries, known answers, borrowed-input
preservation, exact digest binding, concurrency saturation and release, and
cancellation at each context boundary. Semantic fuzzing compares admitted results
with native Argon2 and rejects a one-bit digest mutation. Removing the memory
ceiling fails the retained regression; the unmodified control passes.

Adds the corresponding closed package/error identities and updates Go's extended
crypto dependency to v0.57.0. Release validation and its limitations are retained
separately with the exact source revision and complete command results.
