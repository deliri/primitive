package hostfacts

import (
	"bufio"
	"context"
	"errors"
	"io"
	"math/bits"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

// DiskActivityRequest samples the block device backing Directory. This is
// device busy time and byte throughput, not host-wide pressure or a claim
// about a drive's rated maximum. Unsupported layouts return no observation.
type DiskActivityRequest struct {
	Directory core.AbsolutePath
	Interval  temporal.Duration
}

func (r DiskActivityRequest) Validate() error {
	if r.Interval.IsZero() {
		return core.ErrHostFactsContract
	}
	return errors.Join(r.Directory.Validate(), r.Interval.Validate())
}

// DiskActivity is one observed interval on one native block device. BusyMilli
// is percentage multiplied by 1000: 100000 means 100% busy.
// Kernel accounting is sampled and can miss short bursts; this is not a
// bandwidth benchmark. In-flight includes work from every user of the device.
type DiskActivity struct {
	device        core.AbsolutePath
	window        temporal.Duration
	read, written core.ByteLength
	busy          uint32
	inFlight      uint64
	observed      bool
}

func (o DiskActivity) Validate() error {
	if !o.observed || o.window.IsZero() || o.busy > resourcePercentMilli {
		return core.ErrHostFactsContract
	}
	return errors.Join(o.device.Validate(), o.window.Validate(), o.read.Validate(), o.written.Validate())
}
func (o DiskActivity) Device() core.AbsolutePath     { return o.device }
func (o DiskActivity) Window() temporal.Duration     { return o.window }
func (o DiskActivity) ReadBytes() core.ByteLength    { return o.read }
func (o DiskActivity) WrittenBytes() core.ByteLength { return o.written }
func (o DiskActivity) BusyMilli() uint32             { return o.busy }
func (o DiskActivity) InFlight() uint64              { return o.inFlight }

func ObserveDiskActivity(ctx context.Context, request DiskActivityRequest) (DiskActivity, error) {
	if err := errors.Join(contextstate.Validate(ctx), request.Validate()); err != nil {
		return DiskActivity{}, fail(OperationDiskActivity, core.ErrHostFactsContract, err)
	}
	observation, err := observeDiskActivity(ctx, request)
	if err == nil {
		err = errors.Join(observation.Validate(), contextstate.Validate(ctx))
	}
	if err != nil {
		return DiskActivity{}, fail(OperationDiskActivity, core.ErrHostFactsObservation, err)
	}
	return observation, nil
}

// These are kernel cumulative counters, not a retained device inventory.
type diskActivityCounters struct {
	readSectors, writtenSectors, busyMillis, inFlight uint64
}

func (before diskActivityCounters) delta(device core.AbsolutePath, after diskActivityCounters, window temporal.Duration) (DiskActivity, error) {
	if err := errors.Join(device.Validate(), window.Validate()); err != nil {
		return DiskActivity{}, err
	}
	if window.IsZero() || after.readSectors < before.readSectors || after.writtenSectors < before.writtenSectors || after.busyMillis < before.busyMillis {
		return DiskActivity{}, core.ErrHostFactsObservation
	}
	read, err := blocksToBytes(after.readSectors-before.readSectors, 512)
	if err != nil {
		return DiskActivity{}, err
	}
	written, err := blocksToBytes(after.writtenSectors-before.writtenSectors, 512)
	if err != nil {
		return DiskActivity{}, err
	}
	readBytes, err := core.NewByteLength(read)
	if err != nil {
		return DiskActivity{}, err
	}
	writtenBytes, err := core.NewByteLength(written)
	if err != nil {
		return DiskActivity{}, err
	}
	// Clamp only the physically bounded busy fraction: sampling/jiffies can
	// straddle the clock window. Counter regressions and byte overflow refuse.
	numeratorHigh, numeratorLow := bits.Mul64(after.busyMillis-before.busyMillis, 100000000000)
	denominator := uint64(window.Nanoseconds())
	busy := uint64(resourcePercentMilli)
	if numeratorHigh < denominator {
		busy, _ = bits.Div64(numeratorHigh, numeratorLow, denominator)
		if busy > resourcePercentMilli {
			busy = resourcePercentMilli
		}
	}
	observation := DiskActivity{device: device, window: window, read: readBytes, written: writtenBytes, busy: uint32(busy), inFlight: after.inFlight, observed: true}
	return observation, observation.Validate()
}

// Fold decimal tokens directly from a fixed reader window. Neither the number
// of fields nor leading zeroes makes the retained memory grow.
func parseDiskActivity(ctx context.Context, input io.Reader) (diskActivityCounters, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return diskActivityCounters{}, err
	}
	if core.ReaderIsNil(input) {
		return diskActivityCounters{}, core.ErrHostFactsContract
	}
	reader := bufio.NewReaderSize(input, 4096)
	var out diskActivityCounters
	var value uint64
	index := uint64(0)
	digits := false
	finish := func() {
		if !digits {
			return
		}
		switch index {
		case 2:
			out.readSectors = value
		case 6:
			out.writtenSectors = value
		case 8:
			out.inFlight = value
		case 9:
			out.busyMillis = value
		}
		if index < 11 {
			index++
		}
		value, digits = 0, false
	}
	lineEnded := false
	for {
		if err := contextstate.Validate(ctx); err != nil {
			return diskActivityCounters{}, err
		}
		b, err := reader.ReadByte()
		if errors.Is(err, io.EOF) {
			finish()
			if index < 11 {
				return diskActivityCounters{}, core.ErrHostFactsObservation
			}
			return out, nil
		}
		if err != nil {
			return diskActivityCounters{}, err
		}
		if b == ' ' || b == '\t' || b == '\r' || b == '\n' {
			finish()
			if b == '\n' {
				lineEnded = true
			}
			continue
		}
		if lineEnded {
			return diskActivityCounters{}, core.ErrHostFactsObservation
		}
		if b < '0' || b > '9' || value > (^uint64(0)-uint64(b-'0'))/10 {
			return diskActivityCounters{}, core.ErrHostFactsObservation
		}
		value = value*10 + uint64(b-'0')
		digits = true
	}
}
