package runprotocol

import (
	"bytes"
	"context"
	"errors"
	"io"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lineio"
)

type goFuzzMetricHint uint8

const (
	goFuzzMetricHintExecutions goFuzzMetricHint = iota
	goFuzzMetricHintExecs
	goFuzzMetricHintCrashers
	goFuzzMetricHintInteresting
	goFuzzMetricHintCumulative
	goFuzzMetricHintTotal
	goFuzzMetricHintFailing
	goFuzzMetricHintCount
)

// The filter observes literal presence, not integer or prefix authority.
// Counter admission still runs the owning grammar against the original bytes.
// One complete scan also preserves refusals after an early matching literal.
func goFuzzMetricRecordHints(ctx context.Context, source lineio.RecordSource, span goFuzzMetricSpan) ([goFuzzMetricHintCount]bool, error) {
	var hints [goFuzzMetricHintCount]bool
	literals := [...]string{"executions:", "execs:", "crashers:", "new interesting", "new interesting:", "(total:", "failing input"}
	const lookbehind = len("new interesting:") - 1
	// The window never exceeds the span: a short physical record scans in
	// one read through a buffer of its own size plus the fixed overlap.
	buffer := make([]byte, int(min(max(span.bytes, 1), 32768))+lookbehind)
	guard := &goFuzzFrameSource{ctx: ctx, source: io.NewSectionReader(source, span.offset, span.bytes)}
	carry, empty := 0, 0
	var consumed int64
	for {
		n, readErr := guard.Read(buffer[carry:])
		if n < 0 || n > len(buffer)-carry {
			return [goFuzzMetricHintCount]bool{}, errors.Join(core.ErrGoToolchainOutput, readErr, guard.readErr)
		}
		if n > 0 {
			if int64(n) > span.bytes-consumed {
				return [goFuzzMetricHintCount]bool{}, core.ErrGoToolchainOutput
			}
			consumed += int64(n)
			empty = 0
			window := buffer[:carry+n]
			for index, literal := range literals {
				if goFuzzMetricHint(index) != goFuzzMetricHintFailing && !hints[index] && bytes.Contains(window, []byte(literal)) {
					hints[index] = true
				}
			}
			if !hints[goFuzzMetricHintFailing] {
				// The native spelling uses explicit ASCII case pairs, rather
				// than Unicode case folding. Preserve those exact semantics.
				hints[goFuzzMetricHintFailing] = containsASCIIFolded(window, literals[goFuzzMetricHintFailing])
			}
			carry = min(lookbehind, len(window))
			copy(buffer[:carry], window[len(window)-carry:])
		} else if readErr == nil {
			empty++
			if empty == core.ReaderConsecutiveEmptyReadMaximum {
				return [goFuzzMetricHintCount]bool{}, io.ErrNoProgress
			}
		}
		if readErr != nil {
			if readErr != io.EOF {
				return [goFuzzMetricHintCount]bool{}, errors.Join(readErr, guard.readErr, contextstate.Validate(ctx))
			}
			if err := errors.Join(guard.readErr, contextstate.Validate(ctx)); err != nil {
				return [goFuzzMetricHintCount]bool{}, err
			}
			if consumed != span.bytes {
				return [goFuzzMetricHintCount]bool{}, io.ErrUnexpectedEOF
			}
			return hints, nil
		}
	}
}

// containsASCIIFolded reports whether window, with only ASCII capitals
// lowered, contains the lowercase literal. It compares in place instead of
// copying a folded window.
func containsASCIIFolded(window []byte, literal string) bool {
	for start := 0; start+len(literal) <= len(window); start++ {
		matched := true
		for offset := range len(literal) {
			value := window[start+offset]
			if value >= 'A' && value <= 'Z' {
				value += 'a' - 'A'
			}
			if value != literal[offset] {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}
	return false
}
