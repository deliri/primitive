package runnercontrol

import (
	"bytes"
	"unicode"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/runprotocol"
)

// goBenchmarkPrefix is Go's test2json benchmark name prefix.
const goBenchmarkPrefix = "Benchmark"

// goBenchmarkRows reassembles output lines across consecutive output events
// of one package. Only the pending line's parser and the first field's
// opening bytes are kept; a row longer than any buffer still completes.
type goBenchmarkRows struct {
	row      goBenchmarkStream
	field    [len(goBenchmarkPrefix) + utf8.UTFMax]byte
	fieldLen int
	pkg      [32]byte
	deliver  func(runprotocol.GoBenchmarkRow) error
	pending  bool
	// fieldDone holds once the first whitespace-delimited token has ended.
	fieldDone bool
	resumed   bool
	// attributed holds while the current frame's Action and Package were
	// decoded before its Output; unattributed marks the pending line.
	attributed   bool
	unattributed bool
}

func (*goBenchmarkRows) runnerControlInternalFlow() {}

// write consumes one output event's decoded Output bytes. Rows are delivered
// as their newline is decoded, provisional until the event's OnEvent.
func (r *goBenchmarkRows) write(data []byte, event goJSONProjection) error {
	if !r.resumed {
		r.resumed = true
		known := uint16(1)<<GoEventFieldAction | uint16(1)<<GoEventFieldPackage
		r.attributed = event.fields&known == known && event.packagePresent
		if r.pending && !r.continues(event) {
			if err := r.interrupt(); err != nil {
				return err
			}
		}
	}
	// A decoded non-output Action, such as build-output naming only its
	// ImportPath, never carries a row, whether or not Package was decoded.
	if event.fields&(uint16(1)<<GoEventFieldAction) != 0 && event.action != GoEventActionOutput {
		return nil
	}
	for len(data) > 0 {
		if !r.pending {
			*r = goBenchmarkRows{deliver: r.deliver, pkg: event.packageID, pending: true, resumed: true, attributed: r.attributed, unattributed: !r.attributed}
		}
		line := data
		end := bytes.IndexByte(data, '\n')
		if end >= 0 {
			line = data[:end]
		}
		r.observeField(line)
		r.row.write(line)
		if end < 0 {
			return nil
		}
		data = data[end+1:]
		if err := r.complete(); err != nil {
			return err
		}
	}
	return nil
}

// observeField keeps the first token's opening bytes until its whitespace:
// enough to apply test2json's isBenchmarkName rule without the whole name.
func (r *goBenchmarkRows) observeField(line []byte) {
	if r.fieldDone {
		return
	}
	for _, value := range line {
		if value == ' ' || value == '\t' {
			r.fieldDone = true
			return
		}
		if r.fieldLen < len(r.field) {
			r.field[r.fieldLen] = value
			r.fieldLen++
		}
	}
}

// candidate applies test2json's isBenchmarkName to the first token.
func (r *goBenchmarkRows) candidate() bool {
	field := r.field[:r.fieldLen]
	if !bytes.HasPrefix(field, []byte(goBenchmarkPrefix)) {
		return false
	}
	if len(field) == len(goBenchmarkPrefix) {
		return true
	}
	next, _ := utf8.DecodeRune(field[len(goBenchmarkPrefix):])
	return !unicode.IsLower(next)
}

func (r *goBenchmarkRows) complete() error {
	r.row.endToken()
	row, ok := r.classify()
	r.pending = false
	if !ok {
		return nil
	}
	return r.deliver(row)
}

// classify reports a completed line as a row when it names a benchmark and
// either carries a benchmark unit or has a positive iteration count followed
// by a value, as testing prints even when ReportMetric suppresses ns/op.
// Headers and log lines naming a benchmark stay neutral.
func (r *goBenchmarkRows) classify() (runprotocol.GoBenchmarkRow, bool) {
	b := &r.row
	if !r.candidate() || !b.found && b.metrics == 0 && !(b.iterations && b.valued) {
		return runprotocol.GoBenchmarkRow{}, false
	}
	if r.unattributed {
		return runprotocol.GoBenchmarkRow{Kind: runprotocol.GoBenchmarkRowUnattributed}, true
	}
	switch {
	case b.nameInvalid:
		return runprotocol.GoBenchmarkRow{Kind: runprotocol.GoBenchmarkRowNameUnrepresentable}, true
	case b.malformed || !b.iterations:
		return runprotocol.GoBenchmarkRow{Kind: runprotocol.GoBenchmarkRowMalformed}, true
	case !b.found || b.metrics != 7:
		// Absent ns/op, B/op or allocs/op, including suppressed ns/op.
		return runprotocol.GoBenchmarkRow{Kind: runprotocol.GoBenchmarkRowMetricsMissing}, true
	}
	name, err := runprotocol.NewName(string(b.name[:b.nameLength]))
	if err != nil {
		return runprotocol.GoBenchmarkRow{Kind: runprotocol.GoBenchmarkRowNameUnrepresentable}, true
	}
	measurement := b.measurement
	measurement.Name = name
	if measurement.Validate() != nil {
		return runprotocol.GoBenchmarkRow{Kind: runprotocol.GoBenchmarkRowMalformed}, true
	}
	return runprotocol.GoBenchmarkRow{Kind: runprotocol.GoBenchmarkRowMeasured, Measurement: &measurement}, true
}

// interrupt reports a pending benchmark row that will never complete.
func (r *goBenchmarkRows) interrupt() error {
	candidate := r.pending && r.candidate()
	r.pending = false
	if !candidate {
		return nil
	}
	return r.deliver(runprotocol.GoBenchmarkRow{Kind: runprotocol.GoBenchmarkRowInterrupted})
}

// continues reports whether event may carry the pending line onward: an
// attributed output event of the pending row's own package.
func (r *goBenchmarkRows) continues(event goJSONProjection) bool {
	known := uint16(1)<<GoEventFieldAction | uint16(1)<<GoEventFieldPackage
	return event.fields&known == known && event.packagePresent && !r.unattributed && event.action == GoEventActionOutput && event.packageID == r.pkg
}

// endEvent closes an event: output may resume the pending line in the next
// event of the same package; any other event cuts it, including a frame
// that carried no Output and so never reached write.
func (r *goBenchmarkRows) endEvent(event goJSONProjection) error {
	resumed := r.resumed
	r.resumed = false
	if event.action != GoEventActionOutput || !resumed && !r.continues(event) {
		return r.interrupt()
	}
	return nil
}
