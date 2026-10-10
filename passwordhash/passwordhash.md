# Password hashing execution

`passwordhash` executes Argon2id v19 through Go's maintained `x/crypto/argon2`
implementation. Applications own password enrollment rules, cost selection,
PHC credential encoding, salts, peppers, secret rotation and account lifecycle.
This package has no product defaults and no persistence or network effects.

Construct one `Deriver` per shared resource domain using explicit `Limits`.
Share its pointer across concurrent requests. Constructing a deriver per request
would defeat aggregate admission. `ConcurrentCalls` limits in-flight derivations.
Excess callers wait on Go's admission channel until a slot is released or their
context is cancelled or expires. Ordinary saturation does not refuse requests,
perform automatic retries, weaken Argon2 parameters, or allocate another KDF.
HTTP/platform admission bounds the number of pending request goroutines; the
caller still owns each request's lifetime and resource policy.

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
implementation and creates no timers or background goroutines. The waiting
goroutine belongs to the synchronous caller and exits on its context lifetime.

The ingress inventory is `New`/`Limits.Validate` (caller-owned resource policy),
`Request.Validate`, `Derive`, and `Verify` (borrowed secret material and native
costs). Mechanical boundary tests and `FuzzArgon2idRequestSemanticClosure` use
small, explicitly non-security test costs. Published known answers and direct
Go Argon2 comparisons bind the implementation. Cancellation tests synchronize
at actual context checks; concurrent admission tests hold admitted calls there,
queue excess work, then release real native derivations and verify slot reuse.
Waiter cancellation and deadline tests prove that waiting work publishes no key
and cannot release another caller's occupied slot. Those tests prove ownership
and admission behavior, not application throughput or App Engine capacity.

Product-policy and database integration proof belongs to callers. A local pass
is an execution fact, not an independent acceptance receipt.
