package process

import (
	"context"
	"errors"
	"io"
	"math"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
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

// GroupMemoryRequest borrows host facts for one streamed native ps sample.
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

// GroupUsage is one native sample summed over a group's live members.
type GroupUsage struct {
	Resident core.ByteLength
	CPUTime  temporal.Duration
}

// ObserveGroupUsage reports summed resident bytes and cumulative user+system
// CPU time from one native ps invocation. It is an observation, not
// accounting: ps sees only live members, so CPU time of members that already
// exited is absent: the sum falls when a member exits, and a delta between
// samples can under-count.
// Native RSS counts shared pages in each process separately. Linux reports
// whole seconds, macOS centiseconds. No matching rows means zero. Malformed or
// failed native observations expose no partial sum.
func ObserveGroupUsage(ctx context.Context, request GroupMemoryRequest) (GroupUsage, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return GroupUsage{}, err
	}
	if err := request.Validate(); err != nil {
		return GroupUsage{}, errors.Join(core.ErrProcessContract, err)
	}
	return observeGroupUsage(ctx, request)
}

// ObserveGroupMemory reports the Resident part of ObserveGroupUsage.
func ObserveGroupMemory(ctx context.Context, request GroupMemoryRequest) (core.ByteLength, error) {
	usage, err := ObserveGroupUsage(ctx, request)
	return usage.Resident, err
}

const (
	groupRSSBytesPerKiB             = 1024
	groupCentisecondsPerSecond      = 100
	groupMillisecondsPerCentisecond = 10
	groupSecondsPerDay              = 86400
	groupClockBase                  = 60
	groupHoursPerDay                = 24
	groupClockDigits                = 2
	groupUsageColumns               = 3
	groupTimeColumn                 = 2
	groupTimeMaximumSeparators      = 2
)

// groupUsageFold consumes numeric ps rows "pgid rss time" in fixed space,
// independent of row count, whitespace extent or Write fragmentation. Time is
// procps [DD-]HH:MM:SS or macOS M:SS.cc. Zero group rows are native kernel
// observations; they can never match an actionable group identity.
type groupUsageFold struct {
	target   ProcessIdentity
	resident uint64
	centis   uint64
	group    uint64
	rss      uint64
	number   uint64
	clock    uint64
	lead     uint64
	days     uint64
	width    uint8
	colons   uint8
	column   uint8
	hasDays  bool
	fraction bool
	err      error
}

func (w *groupUsageFold) Write(data []byte) (int, error) {
	if w.err != nil {
		return 0, w.err
	}
	for i, b := range data {
		switch {
		case b >= '0' && b <= '9':
			w.digit(b)
		case b == ':' || b == '-' || b == '.':
			w.mark(b)
		case b == ' ' || b == '\t' || b == '\r' || b == '\n':
			w.endField()
			if b == '\n' {
				w.endRow()
			}
		default:
			w.err = core.ErrProcessObservation
		}
		if w.err != nil {
			return i, w.err
		}
	}
	return len(data), nil
}

func (w *groupUsageFold) digit(b byte) {
	if w.column >= groupUsageColumns || w.number > (math.MaxUint64-uint64(b-'0'))/10 {
		w.err = core.ErrProcessObservation
		return
	}
	w.number = w.number*10 + uint64(b-'0')
	if w.width < math.MaxUint8 {
		w.width++
	}
}

// mark closes one time token. Every token needs digits; minutes and seconds
// after the lead token are two-digit clock values below sixty.
func (w *groupUsageFold) mark(b byte) {
	if w.column != groupTimeColumn || w.width == 0 || w.fraction {
		w.err = core.ErrProcessObservation
		return
	}
	switch b {
	case '-':
		if w.hasDays || w.colons != 0 {
			w.err = core.ErrProcessObservation
			return
		}
		w.days, w.hasDays = w.number, true
	case ':':
		if w.colons >= groupTimeMaximumSeparators || !w.clockToken() {
			return
		}
		if w.colons == 0 {
			w.lead = w.number
		}
		w.colons++
	default:
		if w.colons != 1 || w.hasDays || !w.clockToken() {
			w.err = core.ErrProcessObservation
			return
		}
		w.fraction = true
	}
	w.number, w.width = 0, 0
}

// clockToken folds the current token into clock as the next base-60 place.
func (w *groupUsageFold) clockToken() bool {
	if w.colons != 0 && (w.width != groupClockDigits || w.number >= groupClockBase) ||
		w.clock > (math.MaxUint64-w.number)/groupClockBase {
		w.err = core.ErrProcessObservation
		return false
	}
	w.clock = w.clock*groupClockBase + w.number
	return true
}

func (w *groupUsageFold) endField() {
	if w.err != nil || w.width == 0 && w.column != groupTimeColumn {
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
		}
		w.rss = w.number * groupRSSBytesPerKiB
	case groupTimeColumn:
		if w.width == 0 && w.colons == 0 && !w.hasDays && !w.fraction {
			return
		}
		w.endTime()
	default:
		w.err = core.ErrProcessObservation
	}
	w.column++
	w.number, w.width = 0, 0
}

func (w *groupUsageFold) endTime() {
	fraction := uint64(0)
	if w.fraction {
		if w.width != groupClockDigits {
			w.err = core.ErrProcessObservation
			return
		}
		fraction = w.number
	} else if w.width == 0 || w.colons == 0 || !w.clockToken() ||
		w.colons == groupTimeMaximumSeparators && w.lead >= groupHoursPerDay ||
		w.hasDays && w.colons != groupTimeMaximumSeparators {
		w.err = core.ErrProcessObservation
		return
	}
	if w.days > (math.MaxUint64-w.clock)/groupSecondsPerDay ||
		w.days*groupSecondsPerDay+w.clock > (math.MaxUint64-fraction)/groupCentisecondsPerSecond {
		w.err = core.ErrProcessObservation
		return
	}
	centis := (w.days*groupSecondsPerDay+w.clock)*groupCentisecondsPerSecond + fraction
	w.clock, w.lead, w.days, w.colons, w.hasDays, w.fraction = 0, 0, 0, 0, false, false
	if w.group != uint64(w.target) {
		return
	}
	if w.resident > math.MaxUint64-w.rss || w.centis > math.MaxUint64-centis {
		w.err = core.ErrProcessObservation
		return
	}
	w.resident += w.rss
	w.centis += centis
}

func (w *groupUsageFold) endRow() {
	if w.column != 0 && w.column != groupUsageColumns {
		w.err = core.ErrProcessObservation
	}
	w.column = 0
}

func (w *groupUsageFold) finish() (GroupUsage, error) {
	w.endField()
	w.endRow()
	if w.err != nil {
		return GroupUsage{}, w.err
	}
	resident, err := core.NewByteLength(w.resident)
	if err != nil {
		w.err = errors.Join(core.ErrProcessObservation, err)
		return GroupUsage{}, w.err
	}
	if w.centis > math.MaxUint64/groupMillisecondsPerCentisecond {
		w.err = core.ErrProcessObservation
		return GroupUsage{}, w.err
	}
	cpu, err := temporal.DurationFromMilliseconds(w.centis * groupMillisecondsPerCentisecond)
	if err != nil {
		w.err = errors.Join(core.ErrProcessObservation, err)
		return GroupUsage{}, w.err
	}
	return GroupUsage{Resident: resident, CPUTime: cpu}, nil
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

var _ io.Writer = (*groupUsageFold)(nil)
