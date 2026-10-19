package process

import (
	"context"
	"errors"
	"io"
	"math"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// ObserveGroupLiveness observes a name, not ownership. A reused group identity
// can delay crash recovery; it never grants permission to signal that group.
func ObserveGroupLiveness(group ProcessIdentity) (Liveness, error) {
	if err := validateObservedGroup(group); err != nil {
		return LivenessUnknown, err
	}
	return observedGroupLiveness(group)
}

func validateObservedGroup(group ProcessIdentity) error {
	if err := group.Validate(); err != nil {
		return err
	}
	// -1 names all permitted processes in kill(2), not process group one.
	if group == 1 || uint32(group) > core.ProcessPOSIXIdentityMaximum {
		return contractError(processIdentityDomainDiagnostic)
	}
	return nil
}

// GroupMemoryRequest borrows host facts for one streamed native RSS sample.
// Group remains observation-only, including when reconstructed from a record.
type GroupMemoryRequest struct {
	Group            ProcessIdentity
	WorkingDirectory core.AbsolutePath
	Environment      Environment
}

func (r GroupMemoryRequest) Validate() error {
	if err := errors.Join(validateObservedGroup(r.Group), r.WorkingDirectory.Validate(), r.Environment.Validate()); err != nil {
		return errors.Join(core.ErrProcessContract, err)
	}
	return nil
}

// ObserveGroupMemory reports summed resident bytes, not a memory reservation.
// No matching rows means zero. Malformed or failed native observations expose
// no partial sum. Native RSS counts shared pages in each process separately.
func ObserveGroupMemory(ctx context.Context, request GroupMemoryRequest) (core.ByteLength, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return core.ByteLength{}, err
	}
	if err := request.Validate(); err != nil {
		return core.ByteLength{}, errors.Join(core.ErrProcessContract, err)
	}
	return observeGroupMemory(ctx, request)
}

const groupRSSBytesPerKiB = 1024

// groupMemoryFold consumes numeric ps rows in fixed space, independent of row
// count, whitespace extent or Write fragmentation. Zero group rows are native
// kernel observations; they can never match an actionable group identity.
type groupMemoryFold struct {
	target ProcessIdentity
	total  uint64
	group  uint64
	number uint64
	column uint8
	digits bool
	err    error
}

func (w *groupMemoryFold) Write(data []byte) (int, error) {
	if w.err != nil {
		return 0, w.err
	}
	for i, b := range data {
		if b >= '0' && b <= '9' {
			if w.column >= 2 || w.number > (math.MaxUint64-uint64(b-'0'))/10 {
				w.err = core.ErrProcessObservation
			} else {
				w.number = w.number*10 + uint64(b-'0')
				w.digits = true
			}
		} else if b == ' ' || b == '\t' || b == '\r' || b == '\n' {
			w.endField()
			if b == '\n' {
				w.endRow()
			}
		} else {
			w.err = core.ErrProcessObservation
		}
		if w.err != nil {
			return i, w.err
		}
	}
	return len(data), nil
}

func (w *groupMemoryFold) endField() {
	if !w.digits || w.err != nil {
		return
	}
	switch w.column {
	case 0:
		if w.number > uint64(core.ProcessPOSIXIdentityMaximum) {
			w.err = core.ErrProcessObservation
		}
		w.group = w.number
	case 1:
		if w.number > math.MaxUint64/groupRSSBytesPerKiB {
			w.err = core.ErrProcessObservation
		} else if w.group == uint64(w.target) {
			bytes := w.number * groupRSSBytesPerKiB
			if w.total > math.MaxUint64-bytes {
				w.err = core.ErrProcessObservation
			} else {
				w.total += bytes
			}
		}
	default:
		w.err = core.ErrProcessObservation
	}
	w.column++
	w.number, w.digits = 0, false
}

func (w *groupMemoryFold) endRow() {
	if w.column != 0 && w.column != 2 {
		w.err = core.ErrProcessObservation
	}
	w.column = 0
}

func (w *groupMemoryFold) finish() (core.ByteLength, error) {
	w.endField()
	w.endRow()
	if w.err != nil {
		return core.ByteLength{}, w.err
	}
	length, err := core.NewByteLength(w.total)
	if err != nil {
		w.err = errors.Join(core.ErrProcessObservation, err)
		return core.ByteLength{}, w.err
	}
	return length, nil
}

// OwnedGroup is a non-reconstructible capability held by a process that leads
// its own group. It cannot address a stored identity or another process's group.
type OwnedGroup struct {
	self     *OwnedGroup
	identity ProcessIdentity
}

// OwnGroup acquires only the caller's native group leadership, never a PID
// supplied by a caller. A process sharing another leader's group is refused.
func OwnGroup() (*OwnedGroup, error) {
	identity, err := ownGroupIdentity()
	if err != nil {
		return nil, err
	}
	group := &OwnedGroup{identity: identity}
	group.self = group
	return group, nil
}

func (g *OwnedGroup) Identity() (ProcessIdentity, error) {
	if g == nil || g.self != g {
		return 0, core.ErrProcessContract
	}
	identity, err := ownGroupIdentity()
	if err != nil || identity != g.identity {
		return 0, errors.Join(core.ErrProcessContract, err)
	}
	return identity, nil
}

// Kill hard-stops this process and its group. Success does not return to the
// caller. Zero, copied and no-longer-owned handles fail before any signal.
func (g *OwnedGroup) Kill() error {
	identity, err := g.Identity()
	if err != nil {
		return err
	}
	return sweepGroup(identity)
}

var _ io.Writer = (*groupMemoryFold)(nil)
