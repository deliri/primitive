# Reservation credits

`New(Capacity, Reserve)` constructs a fixed-capacity executor.
`Admit(context.Context, Request)` consumes one credit and returns `Admitted`,
`Exhausted`, or an error. `Exhausted` only follows a successful, exactly bound
zero-credit durable grant. Full local capacity returns
`core.ErrRequestBudgetCapacity` without calling the authority.

The consumer derives `Key` from its namespace, owner and class, supplies a
validated half-open `Window`, an authoritative `Observed` instant, and its
reservation `Batch`. Window and batch remain fixed for a key during a window;
changing either before expiration is a binding error. Observations are trusted
server facts, never client-selected dates. A global observed high-water mark
prevents already-expired windows from reappearing after their slots are reused.
Reordered observations inside a still-live window remain valid.

The synchronous `Reserve` callback receives `Reservation` and returns
`Grant{Reservation: exactRequest, Credits: grantedCount}`. The caller owns the
real database transaction, daily cap, class meaning, routes and HTTP error
translation. Provider errors preserve their identities and never become budget
exhaustion. The callback respects its context and must not recursively call its
executor. Primitive reads no clock and creates no background work.

Same-key calls serialize through completion channels; different keys can reserve
concurrently. Cancellation after a successful durable grant retains every unused
credit. A grant completing after its window has already ended cannot admit paid
work. Live credit and exhaustion slots are never evicted to admit another key;
they are reusable only after their authoritative end. Capacity counts keys,
including each distinct class, rather than owners.

Process death can waste outstanding reserved credits. Consumers must reserve
atomically before granting and must not refund ambiguous calls. The authoritative
daily counter remains the security and cost boundary across process restarts and
multiple instances. Local exhaustion caching applies to this executor instance.

The tests exercise the real local executor with explicitly simulated durable
callbacks. They do not claim Firestore integration. Provider handoff proof
exhausts all 65,536 credit values across matched/foreign binding and success/error
states (262,144 combinations), with an independent arithmetic/error oracle.
Separate proofs cover maximum batches, each binding field, 256 concurrent callers,
capacity refusal, expiry boundaries, window rollback, late grants, waiting
cancellation/deadlines and callback panic cleanup. The AST inventory classifies
all eight production structs and refuses new maps, interfaces or `any`.
