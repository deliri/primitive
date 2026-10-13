// Package background executes caller-owned callbacks on a fixed set of native
// Go workers. Admission is nonblocking and bounds resident callbacks rather
// than lifetime work. Cancellation retires every accepted callback, including
// queued callbacks that never start. Stop reports success only after workers
// and their completion callbacks have returned.
//
// Callback deadlines are cooperative, as in Go's context package. A callback
// that ignores cancellation cannot be forcibly stopped. Stop preserves its
// caller's terminal cause instead of claiming that an incomplete join finished.
// The package stores no identities, schedules, retry policy or durable jobs.
package background
