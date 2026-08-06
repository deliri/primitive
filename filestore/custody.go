package filestore

import (
	"context"
	"errors"
	"os"

	"github.com/deliri/primitive/v2026/contextstate"
)

// Touch stamps one rooted regular file with a custody instant and makes both
// the stamp and the name that carries it durable.
//
// A content-addressed store that meets an object it already holds has nothing
// to write, yet it still has something to record: that the object was wanted
// again at a known time. The modification time is where that fact lives,
// because it is the one piece of custody the filesystem keeps for free and
// reclamation later reads back. Inspection.ModifiedAt already lets a product
// read that instant; without this door there is no way to set it, so every
// product that keeps custody reaches for os.Chtimes on a bare string path and
// leaves the rooted boundary to do it.
//
// Durability is part of the operation rather than a second call. A custody
// stamp that a power cut can erase records nothing, and splitting the sync out
// would hand callers a bare directory sync, which this package refuses to
// expose. A path that is not a regular file is refused before the stamp, so a
// symbolic link or directory planted at the name cannot take the custody
// belonging to the file the caller named.
//
// That refusal is a check on the name, not on the open handle: neither the
// standard library nor this package can stamp a descriptor, so the stamp is
// addressed by name and a rename between the check and the stamp would move
// it. Closing that window needs futimes, and this package is forbidden the
// syscall import that reaches it. The rooted capability still bounds the
// damage to one directory the caller already owns.
func Touch(ctx context.Context, request TouchRequest) error {
	if err := contextstate.Validate(ctx); err != nil {
		return err
	}
	if err := request.Validate(); err != nil {
		return err
	}
	stamp, err := request.ModifiedAt.Time()
	if err != nil {
		return contractError(err)
	}
	root, name := request.Location.Root, request.Location.Path
	file, err := openRegularReadFile(root, name.String())
	if err != nil {
		return err
	}
	if err := root.Chtimes(name.String(), stamp, stamp); err != nil {
		return errors.Join(activationError(err), file.Close())
	}
	syncErr := file.Sync()
	closeErr := file.Close()
	if err := errors.Join(syncErr, closeErr); err != nil {
		return activationError(err)
	}
	if err := syncParent(root, name); err != nil {
		return activationError(err)
	}
	return nil
}

// ConfirmDurable proves one rooted name that is already written will survive
// power loss, and reports the failure to prove it rather than assuming it.
//
// This is not durability for an activation this process performed: Stage,
// Commit, Rename, and Remove each make their own effect durable before
// returning, and a caller that needs those must use them. This answers the
// different question a product asks about a name it did not just write, most
// often after a restart: the record is on disk and its bytes are the expected
// ones, but nothing in this process put it there, so nothing in this process
// knows whether the directory entry naming it ever reached stable storage.
// Re-synchronizing the parent is the only way to find out, and a product
// without this door writes a bare directory sync of its own to do it.
//
// The file itself is opened and confirmed a regular file first. A caller
// asking whether a record is durable is entitled to learn that the name now
// belongs to a directory or a symbolic link instead.
func ConfirmDurable(ctx context.Context, request DurabilityRequest) error {
	if err := contextstate.Validate(ctx); err != nil {
		return err
	}
	if err := request.Validate(); err != nil {
		return err
	}
	root, name := request.Location.Root, request.Location.Path
	file, err := openRegularReadFile(root, name.String())
	if err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return activationError(err)
	}
	if err := syncParent(root, name); err != nil {
		return activationError(err)
	}
	return nil
}

// OpenLockFile opens or creates one rooted file whose only purpose is to carry
// an advisory lock, and hands the caller the real OS handle filelock requires.
//
// filelock.Request takes an *os.File and this package produces one nowhere
// else: OpenRead refuses a file that does not exist yet, and OpenAppend hands
// back an append-only handle a lock holder cannot rewrite its own diagnostics
// through. So every product that takes a file lock opens the file with
// os.OpenFile on a bare string path, which leaves the rooted boundary and
// drags a static-analysis suppression along with it. The two capabilities were
// built to meet and had nothing to meet through.
//
// The handle is opened for reading and writing so the holder can record who it
// is, and created if absent because a lock file's existence is not evidence of
// anything. The caller owns the handle: release the lock, then close it.
func OpenLockFile(ctx context.Context, request LockFileRequest) (*os.File, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return nil, err
	}
	if err := request.Validate(); err != nil {
		return nil, err
	}
	file, err := request.Location.Root.OpenFile(
		request.Location.Path.String(),
		os.O_CREATE|os.O_RDWR,
		request.Mode,
	)
	if err != nil {
		return nil, destinationError(err)
	}
	return file, nil
}
