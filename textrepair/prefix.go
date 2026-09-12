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
	limit := int(min(uint64(len(request.Source)), request.MaximumBytes.Uint64()))
	if limit == 0 {
		return "", nil
	}
	source := request.Source
	if utf8.ValidString(source[:limit]) {
		return source[:limit], nil
	}
	var repaired strings.Builder
	offset, written := 0, 0
	repairing := false
	for offset < len(source) && written < limit {
		r, size := utf8.DecodeRuneInString(source[offset:])
		if r == utf8.RuneError && size == 1 {
			if !repairing {
				repaired.WriteString(source[:offset])
				repairing = true
			}
			offset++
			continue
		}
		if size > limit-written {
			break
		}
		if repairing {
			repaired.WriteString(source[offset : offset+size])
		}
		written += size
		offset += size
	}
	if repairing {
		return repaired.String(), nil
	}
	return source[:offset], nil
}
