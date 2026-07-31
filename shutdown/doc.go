// Package shutdown adds typed, bounded cleanup and signal observation to Go's
// context, time, and os/signal primitives.
//
// A Plan executes compiler-owned phases in order and steps within a phase in
// reverse registration order. Every step and the complete run have explicit
// cooperative budgets. No cleanup callback runs in a helper goroutine: Run
// returns only after every started callback returns.
//
// A Controller owns exactly one os/signal subscription and one goroutine. It
// cancels its context with an authentic SignalCause on the first supported
// signal and can report one escalation fact after a second signal or grace
// expiry. Shutdown never exits a process or runs a force callback; the
// composition root owns that policy.
package shutdown
