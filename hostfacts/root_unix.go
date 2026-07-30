//go:build darwin || linux

package hostfacts

import (
	"errors"
	"os"

	"github.com/deliri/primitive/v2026/core"
	"golang.org/x/sys/unix"
)

type platformRoot struct {
	fd  int
	dev uint64
}

func diskOpenIdentity() core.ErrorIdentity {
	return core.ErrHostFactsObservation
}

func treeOpenIdentity() core.ErrorIdentity {
	return core.ErrHostFactsObservation
}

func openRoot(path string) (*platformRoot, error) {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		if errors.Is(err, unix.ELOOP) || errors.Is(err, unix.ENOTDIR) {
			return nil, errors.Join(core.ErrHostFactsContract, err)
		}
		return nil, err
	}
	var stat unix.Stat_t
	if err := unix.Fstat(fd, &stat); err != nil {
		return nil, errors.Join(err, unix.Close(fd))
	}
	if stat.Mode&unix.S_IFMT != unix.S_IFDIR {
		return nil, errors.Join(core.ErrHostFactsContract, unix.Close(fd))
	}
	return &platformRoot{fd: fd, dev: deviceIdentity(stat)}, nil
}

func (r *platformRoot) close() error {
	if r == nil || r.fd < 0 {
		return core.ErrHostFactsContract
	}
	fd := r.fd
	r.fd = -1
	return unix.Close(fd)
}

func (r *platformRoot) openDirectory(relative string) (*os.File, error) {
	if relative != "." {
		return nil, core.ErrHostFactsContract
	}
	fd, err := unix.Dup(r.fd)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), "."), nil
}

func (r *platformRoot) inspectEntry(parent *os.File, _ string, name string) (treeEntry, error) {
	var before unix.Stat_t
	if err := unix.Fstatat(int(parent.Fd()), name, &before, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return treeEntry{}, err
	}
	switch before.Mode & unix.S_IFMT {
	case unix.S_IFLNK:
		return treeEntry{kind: treeEntryIgnored}, nil
	case unix.S_IFREG:
		return r.inspectRegular(parent, name)
	case unix.S_IFDIR:
		return r.inspectDirectory(parent, name)
	default:
		return treeEntry{kind: treeEntryIgnored}, nil
	}
}

func (r *platformRoot) inspectRegular(parent *os.File, name string) (treeEntry, error) {
	fd, err := unix.Openat(int(parent.Fd()), name, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if err != nil {
		return treeEntry{}, err
	}
	var stat unix.Stat_t
	statErr := unix.Fstat(fd, &stat)
	closeErr := unix.Close(fd)
	if statErr != nil || closeErr != nil {
		return treeEntry{}, errors.Join(statErr, closeErr)
	}
	if stat.Mode&unix.S_IFMT != unix.S_IFREG || deviceIdentity(stat) != r.dev {
		return treeEntry{}, core.ErrHostFactsObservation
	}
	return treeEntry{kind: treeEntryRegular, size: stat.Size}, nil
}

func (r *platformRoot) inspectDirectory(parent *os.File, name string) (treeEntry, error) {
	fd, err := unix.Openat(int(parent.Fd()), name, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return treeEntry{}, err
	}
	var stat unix.Stat_t
	if err := unix.Fstat(fd, &stat); err != nil {
		return treeEntry{}, errors.Join(err, unix.Close(fd))
	}
	if stat.Mode&unix.S_IFMT != unix.S_IFDIR || deviceIdentity(stat) != r.dev {
		return treeEntry{}, errors.Join(core.ErrHostFactsObservation, unix.Close(fd))
	}
	return treeEntry{kind: treeEntryDirectory, directory: os.NewFile(uintptr(fd), name)}, nil
}
