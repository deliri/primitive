// Package process runs one typed command over Go's os/exec primitive.
//
// Input and output remain caller-owned streams. Process forwards bytes without
// retaining whole output, applies one fixed bound independently to stdout and
// stderr, and reports the direct child's exit and resource observations.
//
// Process does not interpret a shell language, construct pipelines, schedule
// work, retain a process registry, or supervise a process tree. Callers that
// require process-group, job-object, second-signal, or descendant policy own
// that composition above this package.
//
// Two boundaries belong to the caller and are not defects in this package.
//
// Request.WaitDelay bounds only the descriptors os/exec owns. When a child
// exits while a descendant still holds an inherited output descriptor, Wait
// closes the parent pipes after the delay and Run returns a FailureKindWait
// Failure whose cause preserves the native exec.ErrWaitDelay. When a
// caller-supplied Reader or Writer blocks indefinitely instead, closing those
// pipes cannot unblock it; Process kills and reaps the direct child, and
// releasing a caller-owned stream implementation stays with the caller.
//
// A child that completes before an observed cancellation is a successful
// observation. Cancellation is reported only when this package's kill actually
// signaled the direct child, and that report preserves both the context cause
// and the native wait error.
package process
