//go:build aix || android || darwin || dragonfly || freebsd || illumos || ios || linux || netbsd || openbsd || solaris

package filestore

import (
	"io/fs"
	"syscall"
)

// observedOwnership reads the numeric owner out of the observation this
// package already made.
//
// This is a type assertion on a value Lstat already returned, not a call. The
// distinction is the whole reason the import is admissible here: nothing in
// this file asks the kernel for anything, so no unrooted path access and no
// second observation is introduced, and the identifiers describe the same
// entry the rest of the Inspection describes rather than whatever occupies the
// name a moment later.
//
// A rooted alternative would be better and does not exist. os.Root exposes no
// descriptor, so Fstatat against the capability cannot be reached, and the
// only other route is a fresh Lstat on the absolute path, which abandons the
// confinement Inspect performs the observation through.
//
// A host whose FileInfo carries something else answers "unobserved" rather
// than zero, so a caller is never told root owns a file on a platform with no
// numeric owners at all.
func observedOwnership(info fs.FileInfo) Ownership {
	status, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return Ownership{}
	}
	return Ownership{uid: uint32(status.Uid), gid: uint32(status.Gid), set: true}
}
