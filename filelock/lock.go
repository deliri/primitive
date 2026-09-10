package filelock

import (
	"context"
	"errors"
	"os"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// Acquire takes one advisory lock on the requested file.
//
// An Immediate attempt that finds another holder returns a valid Acquisition
// reporting held=false. That is the whole reason the outcome is typed: a
// caller must be able to tell "another process is running" from "locking does
// not work on this filesystem", and an implementation that collapses both into
// an error turns a real failure into a timeout with its cause erased.
//
// A Blocking attempt returns only when it holds the lock or fails. Context is
// validated on entry, but no cancellation can reach a process parked in the
// lock call, so a caller that must remain interruptible uses Immediate and
// owns its own retry cadence. The Go descriptor reference remains pinned during
// a blocking call; closing that same handle waits for this call to finish.
func Acquire(ctx context.Context, request Request) (Acquisition, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return Acquisition{}, err
	}
	if err := request.Validate(); err != nil {
		return Acquisition{}, err
	}
	var held bool
	err := controlFile(request.File, func(fd uintptr) error {
		var err error
		held, err = acquire(fd, request.Exclusivity, request.Patience)
		return err
	})
	if err != nil {
		return Acquisition{}, lockError(err)
	}
	return newAcquisition(held), nil
}

// Release drops the advisory lock held on file.
//
// Closing the descriptor also releases the lock, and the operating system does
// it if the process dies. Releasing explicitly is for a caller that keeps the
// file open for other work after it is done excluding others.
func Release(ctx context.Context, file *os.File) error {
	if err := contextstate.Validate(ctx); err != nil {
		return err
	}
	if file == nil {
		return contractError(errors.New(fileMissingDiagnostic))
	}
	if err := controlFile(file, release); err != nil {
		return lockError(err)
	}
	return nil
}

func contractError(err error) error {
	return errors.Join(core.ErrPrimitiveContract, err)
}

func lockError(err error) error {
	return errors.Join(core.ErrFileLockUnavailable, err)
}

// controlFile pins descriptor lifetime through Go and leaves its poller mode intact.
func controlFile(file *os.File, effect func(uintptr) error) error {
	conn, err := file.SyscallConn()
	if err != nil {
		return err
	}
	var effectErr error
	controlErr := conn.Control(func(fd uintptr) { effectErr = effect(fd) })
	return errors.Join(controlErr, effectErr)
}
