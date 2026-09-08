package filestore_test

import (
	"errors"
	"io"
	"os"
	"path/filepath"
)

type heldNativeMutation uint8

const (
	heldNativeUnchanged heldNativeMutation = iota
	heldNativeHardLink
	heldNativeRenamedCandidate
	heldNativeForeignSameBytes
	heldNativeReplacementDirectory
	heldNativeLinkToOwned
	heldNativeDanglingLink
	heldNativeOutsideLink
	heldNativeUnlinked
	heldNativeRenamedAway
	heldNativeMissingParent
	heldNativeFileParent
	heldNativeAncestorAlias
	heldNativeAncestorOutside
	heldNativeAncestorCycle
	heldNativeNilHandle
	heldNativeClosedHandle
	heldNativeZeroPath
	heldNativeNilContext
	heldNativeCanceledContext
	heldNativeDirectory
	heldNativePipe
	heldNativeMutationLimit
)

type heldNativeFixture struct {
	file   *os.File
	path   string
	closed bool
}

// Native fixture construction only. The returned handle, namespace and mutation
// remain visible to independent Go observations in the table/fuzz executor.
func createHeldNativeFixture(container string, mutation heldNativeMutation, payload []byte) (fixture heldNativeFixture, err error) {
	parent := filepath.Join(container, "parent")
	outside := filepath.Join(container, "outside")
	for _, directory := range []string{parent, outside} {
		if err = os.Mkdir(directory, 0o700); err != nil {
			return fixture, err
		}
	}
	for _, name := range []string{filepath.Join(outside, "held"), filepath.Join(parent, "neighbor")} {
		if err = os.WriteFile(name, payload, 0o600); err != nil {
			return fixture, err
		}
	}
	fixture.path = filepath.Join(parent, "held")
	if mutation == heldNativeDirectory {
		if err = os.Mkdir(fixture.path, 0o700); err != nil {
			return fixture, err
		}
		if err = os.WriteFile(filepath.Join(fixture.path, "child"), payload, 0o600); err != nil {
			return fixture, err
		}
	} else if err = os.WriteFile(fixture.path, payload, 0o600); err != nil {
		return fixture, err
	}
	if mutation == heldNativePipe {
		var writer *os.File
		fixture.file, writer, err = os.Pipe()
		if err != nil {
			return fixture, err
		}
		written, writeErr := writer.Write(payload)
		closeErr := writer.Close()
		if writeErr == nil && written != len(payload) {
			writeErr = io.ErrShortWrite
		}
		err = errors.Join(writeErr, closeErr)
		if err != nil {
			_ = fixture.file.Close()
			return fixture, err
		}
	} else {
		fixture.file, err = os.Open(fixture.path)
		if err != nil {
			return fixture, err
		}
	}
	defer func() {
		if err != nil && !fixture.closed {
			_ = fixture.file.Close()
		}
	}()
	archive := filepath.Join(parent, "archive")
	switch mutation {
	case heldNativeUnchanged, heldNativeNilHandle, heldNativeZeroPath, heldNativeNilContext, heldNativeCanceledContext, heldNativeDirectory, heldNativePipe:
	case heldNativeHardLink:
		alias := filepath.Join(parent, "alias")
		err = os.Link(fixture.path, alias)
		fixture.path = alias
	case heldNativeRenamedCandidate:
		err = os.Rename(fixture.path, archive)
		fixture.path = archive
	case heldNativeForeignSameBytes, heldNativeReplacementDirectory, heldNativeLinkToOwned, heldNativeDanglingLink, heldNativeOutsideLink:
		if err = os.Rename(fixture.path, archive); err != nil {
			return fixture, err
		}
		switch mutation {
		case heldNativeForeignSameBytes:
			err = os.WriteFile(fixture.path, payload, 0o600)
		case heldNativeReplacementDirectory:
			err = os.Mkdir(fixture.path, 0o700)
		case heldNativeLinkToOwned:
			err = os.Symlink("archive", fixture.path)
		case heldNativeDanglingLink:
			err = os.Symlink("missing", fixture.path)
		case heldNativeOutsideLink:
			err = os.Symlink(filepath.Join(outside, "held"), fixture.path)
		default:
			return fixture, os.ErrInvalid
		}
	case heldNativeUnlinked:
		err = os.Remove(fixture.path)
	case heldNativeRenamedAway:
		err = os.Rename(fixture.path, archive)
	case heldNativeMissingParent, heldNativeFileParent:
		err = os.Rename(parent, filepath.Join(container, "archive"))
		if err == nil && mutation == heldNativeFileParent {
			err = os.WriteFile(parent, payload, 0o600)
		}
	case heldNativeAncestorAlias, heldNativeAncestorOutside, heldNativeAncestorCycle:
		alias := filepath.Join(container, "alias")
		target := parent
		if mutation == heldNativeAncestorOutside {
			target = outside
		} else if mutation == heldNativeAncestorCycle {
			target = alias
		}
		err = os.Symlink(target, alias)
		fixture.path = filepath.Join(alias, "held")
	case heldNativeClosedHandle:
		err = fixture.file.Close()
		fixture.closed = true
	default:
		return fixture, os.ErrInvalid
	}
	return fixture, err
}
