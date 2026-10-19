// Package textrepair provides bounded UTF-8 repair through Go's UTF-8 decoder.
package textrepair

import (
	"strings"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
)

// Request selects the output extent of a repaired UTF-8 prefix.
// Malformed source bytes are discarded; a genuine U+FFFD is retained.
// MaximumBytes bounds output and scratch, not the number of bytes scanned.
type Request struct {
	// Source is caller-owned text; malformed bytes are permitted and discarded.
	Source string
	// MaximumBytes bounds emitted UTF-8 bytes; zero permits no output.
	MaximumBytes core.ByteLength
}

// Validate checks the output extent before any source work.
func (r Request) Validate() error { return r.MaximumBytes.Validate() }

// Prefix returns the longest ordered prefix of repaired source that
// fits the byte budget. A valid rune that does not fit terminates the prefix;
// later, smaller runes cannot jump ahead of it. Zero budget yields empty output.
// Valid bounded input is borrowed without allocation; repaired output owns its
// storage. Callers requiring independent retention should copy borrowed output.
func Prefix(request Request) (string, error) {
	if err := request.Validate(); err != nil {
		return "", err
	}
	// Both operands are nonnegative and min is at most len(Source), an int.
	limit := int(min(uint64(len(request.Source)), request.MaximumBytes.Uint64())) // #nosec G115 -- The source's native int length proves this narrowing conversion fits on every target architecture.
	if limit == 0 {
		return "", nil
	}
	source := request.Source
	if utf8.ValidString(source[:limit]) {
		return source[:limit], nil
	}
	return repairPrefix(source, limit), nil
}

// repairPrefix copies each maximal run of admitted runes in one write. Output
// storage is reserved once at the limit, which already bounds output and
// scratch, so repair never regrows its buffer.
func repairPrefix(source string, limit int) string {
	var repaired strings.Builder
	offset, written, run := 0, 0, 0
	repairing := false
	for offset < len(source) && written < limit {
		size := 1
		if source[offset] >= utf8.RuneSelf {
			var r rune
			r, size = utf8.DecodeRuneInString(source[offset:])
			if r == utf8.RuneError && size == 1 {
				if !repairing {
					repaired.Grow(limit)
					repairing = true
				}
				repaired.WriteString(source[run:offset])
				offset++
				run = offset
				continue
			}
		}
		if size > limit-written {
			break
		}
		written += size
		offset += size
	}
	if repairing {
		repaired.WriteString(source[run:offset])
		return repaired.String()
	}
	return source[:offset]
}
