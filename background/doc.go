// Package background executes caller-owned callbacks on native Go workers or
// one parent-owned Task. Worker admission is nonblocking and bounds resident callbacks rather
// than lifetime work. Cancellation retires every accepted callback, including
// queued callbacks that never start. Stop reports success only after workers
// and their completion callbacks have returned.
//
// Callback deadlines are cooperative, as in Go's context package. A callback
// that ignores cancellation cannot be forcibly stopped. Stop preserves its
// caller's terminal cause instead of claiming that an incomplete join finished.
// StartTask owns one action and completion observer without imposing an
// independent operation timeout. Its parent owns that lifetime; Wait can bound
// a join without canceling the action, and Close cancels and joins cooperatively.
// The package stores no identities, schedules, retry policy or durable jobs.
package background
