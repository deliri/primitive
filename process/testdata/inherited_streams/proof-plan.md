A relay child must reach its parent's standard streams while its own belong to
someone else: rsync gives its remote-shell child a private socket, so a bridge
reaching the SSH session around it can only find that session on a lent
descriptor. Request.Inherited lends only the caller's own standard streams, at
descriptors 3 onward in order; LentStreams opens them in the child. The fd
mapping lives only here. The two-way copy is filestore.Relay, because process
owns no goroutines.

Three real processes run per row through process.Run: the test, the lending
parent and the relay child, which composes StandardStreams, LentStreams and
filestore.Relay exactly as a consumer bridge does. Rows cover both lending
orders, half a megabyte each way, an echo row whose upward bytes are the
downward bytes returned, empty streams, and an unlent child whose relay fails
with the native bad-descriptor cause when those descriptors are absent.
LentStreams performs no filesystem effect. Its trusted parent and child must
agree on descriptor inheritance/order: unrelated open descriptors could use
the same numbers, so this API does not authenticate descriptor origin.
The domain test is exhaustive over 256 values; the request triad covers
neutral, single, both orders, unknown, future, repeated and over-count lending.

Each declared mutation must fail its named row with an assertion, not a build
failure or panic. An irregular-descriptor check that no row could distinguish,
and a Stat refusal Witness classified as a filesystem effect, were removed. Non-Unix hosts refuse lending with
ErrProcessUnsupported; that leaf is compiled for windows but not executed.
