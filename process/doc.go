// Package process runs one typed command over Go's os/exec primitive.
//
// Input and output remain caller-owned streams. Process forwards bytes without
// retaining whole output, applies one fixed bound independently to stdout and
// stderr, and reports the direct child's exit and resource observations.
//
// Process does not interpret a shell language, construct pipelines, schedule
// work, or retain a process registry. It does own the isolation and signal a
// caller declares in Request.Containment: on POSIX hosts a child may lead its
// own process group so a cancellation addresses the whole tree (Windows has
// no group signal and refuses that request rather than delivering it to one
// process), and the cancel signal is a closed choice. A fully zero
// Containment names the documented direct-kill default. Begin hands back a running
// Execution a supervisor holds to signal, terminate, or interrogate the child
// while the reaping wait is in flight; Alive answers whether one identity
// still names a process; Sweep hard-stops every survivor in a group-contained
// tree, before or after the leader is reaped, treating a group already gone
// as success. Registry and force-drain policy above one child stay with the
// caller.
//
// AmbientEnvironment admits the complete inherited environment when a caller
// must filter it for a child. LookupAmbientEnvironment observes one exact
// variable in O(1) memory and preserves the difference between absence and a
// present empty value.
//
// ExitCommand is the one ambient termination door. It accepts a closed
// ExitStatus and belongs only at a package-main boundary; libraries return
// typed failures and never terminate their host process.
// DiscardDeviceArgument exposes the platform null device as a validated argv
// value for compiler and linker outputs that are intentionally not retained.
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
