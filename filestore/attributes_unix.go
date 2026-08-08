//go:build aix || android || darwin || dragonfly || freebsd || illumos || ios || linux || netbsd || openbsd || solaris

package filestore

import (
	"errors"
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

// errnoSaysNotADirectory names the errno POSIX kernels answer when an
// intermediate path component is occupied by something that is not a
// directory. It is a constant comparison against an error the standard
// library already returned, not a call, which is why it may live beside the
// other reads in this one syscall-naming leaf.
func errnoSaysNotADirectory(err error) bool {
	return errors.Is(err, syscall.ENOTDIR)
}

// observedAllocation reads the allocated block count out of the same
// already-returned structure, under the same admissibility argument.
//
// Every platform in this leaf's build tag defines st_blocks in 512-byte units
// (the historical DEV_BSIZE convention; POSIX itself leaves the unit
// implementation-defined), so the projection to bytes is a constant, not a
// second observation. Unreported stays honest only for a host whose FileInfo
// carries no Stat_t at all. A filesystem that did report and reported garbage,
// a negative count or one too large to express as a byte length, is a refusal
// like the sibling size fact: unreported is the vacuously satisfied answer on
// the reserve question this door exists to decide, and garbage must not flow
// toward acceptance.
func observedAllocation(info fs.FileInfo) (Allocation, error) {
	status, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return Allocation{}, nil
	}
	if status.Blocks < 0 {
		return Allocation{}, sourceError(errors.New("filesystem reported a negative allocated block count"))
	}
	if uint64(status.Blocks) > math.MaxUint64/512 {
		return Allocation{}, sourceError(errors.New("filesystem reported an unrepresentable allocated block count"))
	}
	bytes, err := core.NewByteLength(uint64(status.Blocks) * 512)
	if err != nil {
		return Allocation{}, contractError(err)
	}
	return Allocation{bytes: bytes, reported: true}, nil
}
