# Dedicated terminal relay proof

Assumptions: rsync closes its rsh stdin after waiting for the child (confirmed
from retained native hangs and upstream process setup); a Go os.File close
always interrupts a blocking inherited read (false); a library can return with
that borrowed read alive (false). Descriptor numbers prove origin (false:
trusted parent/child lending is a precondition). The bridge process has no
unrelated work/deferred cleanup (required caller precondition, not introspectable).

Facts: generic Relay379 joins both copies. Upstream rsync pipe.c creates two
socketpairs and makes child stdin blocking. A dedicated process can terminate
its own blocking read through existing Process.ExitCurrent; its parent must
reap it. Current stdout writes pass through Process.WriteOutput so EPIPE cannot
preempt native-cause diagnostics with SIGPIPE.

Smallest cut: a terminal Filestore entrypoint taking LentRequest, current
standard/lent handles only, one copy goroutine and one capacity-one channel.
Copy completion precedes output close; a known copy joins that close. Far EOF
exits the dedicated process. No RelayResult or joined claim is returned. Known
copy/close errors reach stderr and ExitFailure. Arbitrary native streams exist
only in the private I/O scope for hostile owned-child failure rows.

The native harness holds stdin open, acknowledges upward bytes before Far EOF,
checks exact downward bytes in both descriptor orders, proves no continuation
or defer runs, and reaps the process with native group ESRCH. Failure rows
causally observe upward output EOF before releasing Far input, conserving copy
and close causes. EPIPE must be an explicit failure exit rather than a signal.
Cancellation begins after actual downward bytes and joins the group. Failed
backstops release and join before reporting failure.
The closed-output oracle obtains Go RawControl's actual native cause rather
than assuming every closed descriptor is os.ErrClosed. A broken stderr is
also challenged: diagnostics cannot be delivered, but exit must remain an
explicit failure rather than SIGPIPE or success.

Seven compiling patches separately challenge early library return, joining
blocked stdin, each known error edge, native stdout SIGPIPE handling and wrong
success status. Build failures do not count as kills. Structural adversaries
reject public/arbitrary-stream owners, unbounded channels, worker pools,
post-start returns, invalid exit values, callbacks and a missing close join.
Existing generic Relay379 proof is unchanged and reused.

Linux evidence is collected first. Darwin execution and actual Anvil rsync
integration remain unverified until their owners retain matching native proof.
A closed stderr cannot deliver diagnostics; the process still exits failure.
An unobserved upward copy ends with process lifetime, without a success receipt.
