# Password hashing execution

`passwordhash` executes Argon2id v19 through Go's maintained `x/crypto/argon2`
implementation. Applications own password enrollment rules, cost selection,
PHC credential encoding, salts, peppers, secret rotation and account lifecycle.
This package has no product defaults and no persistence or network effects.

Construct one `Deriver` per shared resource domain using explicit `Limits`.
Share its pointer across concurrent requests. Constructing a deriver per request
would defeat aggregate admission. `ConcurrentCalls` limits in-flight derivations;
there is no waiter queue and no automatic retry. Saturation returns the stable
`core.ErrPasswordHashCapacity`. Callers decide the external busy response.

`Request` borrows exact material and salt bytes until the synchronous operation
returns. Callers own their contents and clearing. `Derive` returns a caller-owned
key; clear it after use. `Verify` compares an exact-length digest in constant time
and clears its temporary candidate. A mismatch is false with no error.

Costs and lengths must fit both the native Argon2 domain and the caller's policy.
The caller sets maximum KiB, iterations, parallelism, material, salt and key bytes.
These bounds are explicit application resource authorization, not library-chosen
stream transfer quotas. Password hashing is a whole-value KDF; it does not claim
streaming execution. Maximum native work per shared domain is bounded by admitted
concurrency times caller-selected per-operation cost, plus runtime overhead.

Cancellation is checked before admission, after admission and before publication.
Go's Argon2 call cannot be interrupted midway. An admitted call retains its slot
until native work ends; cancellation then clears the key and returns no result.
The package launches no additional workers beyond those inside Go's Argon2
implementation and creates no timers, queues or background goroutines.

The ingress inventory is `New`/`Limits.Validate` (caller-owned resource policy),
`Request.Validate`, `Derive`, and `Verify` (borrowed secret material and native
costs). Mechanical boundary tests and `FuzzArgon2idRequestSemanticClosure` use
small, explicitly non-security test costs. Published known answers and direct
Go Argon2 comparisons bind the implementation. Cancellation tests synchronize
at actual context checks; concurrent admission tests hold admitted calls there,
refuse excess work, then release real native derivations and verify slot reuse.
Those tests prove ownership and refusal, not application throughput or latency.

Product-policy and database integration proof belongs to callers. A local pass
is an execution fact, not an independent acceptance receipt.
