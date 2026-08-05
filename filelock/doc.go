// Package filelock takes one advisory lock on one already-open file.
//
// Advisory locking is the only way separate processes agree on who owns a
// directory of work, and Go's standard library does not expose it. Every
// product therefore grows its own copy over syscall or golang.org/x/sys, and
// each copy makes the same three decisions slightly differently: which errors
// mean "somebody else holds it" rather than "this failed", whether an
// interrupted call retries, and whether the caller can tell those apart at
// all. Getting the first one wrong turns a real failure into a timeout whose
// cause is lost.
//
// The lock is the descriptor. The file's contents are not the lock, and no PID
// written into it establishes ownership: process identifiers are reused, so a
// stale number names a live stranger. Closing the descriptor releases the lock,
// and the operating system releases it if the process dies, which is what makes
// it safe against a crash that never ran cleanup.
//
// The file itself belongs to the caller. Opening it, creating it, writing
// diagnostics into it, and closing it are the caller's, because a lock file's
// name, permissions, and contents are product decisions. This package only
// decides who holds it.
package filelock
