//go:build aix || android || darwin || dragonfly || freebsd || illumos || ios || linux || netbsd || openbsd || solaris

package filestore

import (
	"io/fs"
	"math"
	"syscall"

	"github.com/deliri/primitive/v2026/core"
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

// observedAllocation reads the allocated block count out of the same
// already-returned structure, under the same admissibility argument.
//
// POSIX fixes st_blocks in 512-byte units regardless of the filesystem's
// preferred block size, so the projection to bytes is a constant, not a
// second observation. A negative count, or one too large to express as a
// byte length, is answered as unreported rather than as a fabricated value.
func observedAllocation(info fs.FileInfo) Allocation {
	status, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return Allocation{}
	}
	if status.Blocks < 0 || uint64(status.Blocks) > math.MaxUint64/512 {
		return Allocation{}
	}
	bytes, err := core.NewByteLength(uint64(status.Blocks) * 512)
	if err != nil {
		return Allocation{}
	}
	return Allocation{bytes: bytes, reported: true}
}
