package hostfacts

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

const (
	rotationUnsupportedLabel   = "unsupported"
	rotationUnavailableLabel   = "unavailable"
	rotationRotationalLabel    = "rotational"
	rotationNonRotationalLabel = "non-rotational"

	rotationalFlagRotationalToken    = "1"
	rotationalFlagNonRotationalToken = "0"
	// rotationalFlagMaximumBytes bounds the kernel's one-token declaration
	// with room for no representation beyond the documented token and newline.
	rotationalFlagMaximumBytes = 16
)

// DiskRotation is the closed set of answers to one capacity-planning
// question: does the block device backing a directory rotate?
//
// Closed rather than boolean because a host can decline to answer two
// different ways: an operating system with no portable interface is
// unsupported, and a directory no single block device backs, such as an
// overlay or a network mount, is unavailable. A caller planning I/O treats
// both as unknown; a caller recording host facts records which one it
// observed.
type DiskRotation uint8

const (
	// DiskRotationUnknown is outside the admitted domain.
	DiskRotationUnknown DiskRotation = iota
	// DiskRotationUnsupported reports an operating system with no portable
	// rotation interface.
	DiskRotationUnsupported
	// DiskRotationUnavailable reports a host that supports the interface
	// but names no single block device behind this directory.
	DiskRotationUnavailable
	// DiskRotationRotational reports a spinning device.
	DiskRotationRotational
	// DiskRotationNonRotational reports a device that does not spin.
	DiskRotationNonRotational
	diskRotationLimit
)

func diskRotationLabels() [diskRotationLimit]string {
	return [...]string{
		DiskRotationUnsupported:   rotationUnsupportedLabel,
		DiskRotationUnavailable:   rotationUnavailableLabel,
		DiskRotationRotational:    rotationRotationalLabel,
		DiskRotationNonRotational: rotationNonRotationalLabel,
	}
}

// Validate rejects rotations outside the closed domain.
func (r DiskRotation) Validate() error {
	if !r.IsValid() {
		return errors.Join(core.ErrHostFactsContract, errors.New("disk rotation is outside the closed domain"))
	}
	return nil
}

// IsValid reports membership in the closed rotation domain.
func (r DiskRotation) IsValid() bool {
	return r > DiskRotationUnknown && r < diskRotationLimit && diskRotationLabels()[r] != ""
}

// OffWireEnum declares DiskRotation as a runtime observation rather than a
// wire encoding.
func (DiskRotation) OffWireEnum() {}

// String returns the compiler-owned diagnostic label for r.
func (r DiskRotation) String() string {
	if !r.IsValid() {
		return core.UnknownEnumDiagnostic
	}
	return diskRotationLabels()[r]
}

// DiskRotationRequest binds the observation to one exact directory.
type DiskRotationRequest struct {
	Directory core.AbsolutePath
}

// Validate rejects an unset or malformed directory.
func (r DiskRotationRequest) Validate() error {
	if err := r.Directory.Validate(); err != nil {
		return errors.Join(core.ErrHostFactsContract, err)
	}
	return nil
}

// classifyRotationalFlag admits the kernel's rotational declaration. The
// interface publishes exactly one token, zero or one, with one trailing
// newline; anything else is a source that stopped speaking the documented
// interface, and that is a failed observation rather than a guess.
func classifyRotationalFlag(data []byte) (DiskRotation, error) {
	if len(data) == 0 || len(data) > rotationalFlagMaximumBytes {
		return DiskRotationUnknown, core.ErrHostFactsObservation
	}
	token, err := canonicalVirtualValueToken(data)
	if err != nil {
		return DiskRotationUnknown, err
	}
	switch token {
	case rotationalFlagNonRotationalToken:
		return DiskRotationNonRotational, nil
	case rotationalFlagRotationalToken:
		return DiskRotationRotational, nil
	default:
		return DiskRotationUnknown, errors.Join(
			core.ErrHostFactsObservation,
			errors.New("rotational flag is outside the documented domain"),
		)
	}
}
