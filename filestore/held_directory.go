package filestore

import (
	"context"
	"errors"
	"os"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// FilesystemIdentity is the operating system's identity for one filesystem.
// It is an opaque comparison coordinate, not a device-name convention.
type FilesystemIdentity struct {
	value uint64
	valid bool
}

func newFilesystemIdentity(value uint64) FilesystemIdentity {
	return FilesystemIdentity{value: value, valid: true}
}

// Validate rejects an identity that was not observed by Filestore.
func (i FilesystemIdentity) Validate() error {
	if !i.valid {
		return contractError(errors.New("filestore filesystem identity is unset"))
	}
	return nil
}

// Uint64 returns the host coordinate without assigning product meaning to it.
func (i FilesystemIdentity) Uint64() (uint64, error) {
	if err := i.Validate(); err != nil {
		return 0, err
	}
	return i.value, nil
}

// HeldDirectory owns one exact open directory and its observed filesystem
// identity. The concrete Go handle remains available for a cooperating
// Primitive capability that must perform a host-specific query against the
// already-open object.
type HeldDirectory struct {
	file       *os.File
	filesystem FilesystemIdentity
}

// Validate rejects an absent handle or unobserved filesystem identity.
func (d HeldDirectory) Validate() error {
	if d.file == nil {
		return contractError(errors.New(heldDirectoryAbsentDiagnostic))
	}
	return d.filesystem.Validate()
}

// File returns the borrowed standard-library handle. HeldDirectory retains
// ownership; callers close the HeldDirectory rather than the borrowed file.
func (d HeldDirectory) File() (*os.File, error) {
	if err := d.Validate(); err != nil {
		return nil, err
	}
	return d.file, nil
}

// Filesystem returns the exact filesystem identity observed during open.
func (d HeldDirectory) Filesystem() (FilesystemIdentity, error) {
	if err := d.Validate(); err != nil {
		return FilesystemIdentity{}, err
	}
	return d.filesystem, nil
}

// Close releases the held directory exactly once.
func (d *HeldDirectory) Close() error {
	if d == nil || d.file == nil {
		return contractError(errors.New(heldDirectoryAbsentDiagnostic))
	}
	file := d.file
	d.file = nil
	if err := file.Close(); err != nil {
		return sourceError(err)
	}
	return nil
}

// OpenDirectory opens one exact directory without following its final path
// component and records the filesystem identity from that same handle.
func OpenDirectory(ctx context.Context, path core.AbsolutePath) (*HeldDirectory, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return nil, err
	}
	if err := path.Validate(); err != nil {
		return nil, contractError(err)
	}
	file, filesystem, err := openHeldDirectory(path.String())
	if err != nil {
		return nil, sourceError(err)
	}
	directory := &HeldDirectory{file: file, filesystem: filesystem}
	if err := directory.Validate(); err != nil {
		return nil, errors.Join(err, file.Close())
	}
	return directory, nil
}

var (
	_ core.Validatable = FilesystemIdentity{}
	_ core.Validatable = HeldDirectory{}
)
