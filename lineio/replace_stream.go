package lineio

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"math"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// LiteralReplacement supplies one exact byte substitution. Empty replacement
// deletes matches. Borrowed bytes must remain immutable until execution returns.
type LiteralReplacement struct {
	Pattern     []byte
	Replacement []byte
}

func (r LiteralReplacement) Validate() error {
	if len(r.Pattern) == 0 {
		return core.ErrLineIOContract
	}
	return nil
}

// ReplaceStreamRequest supplies native stream capabilities and working memory.
// BufferBytes controls the read window, never the source or output extent.
type ReplaceStreamRequest struct {
	Source      io.Reader
	Destination io.Writer
	Literal     LiteralReplacement
	BufferBytes core.ByteCount
}

func (r ReplaceStreamRequest) Validate() error {
	if core.ReaderIsNil(r.Source) || core.WriterIsNil(r.Destination) {
		return core.ErrLineIOContract
	}
	if err := r.Literal.Validate(); err != nil {
		return err
	}
	size, err := r.BufferBytes.Uint64()
	if err != nil || size > math.MaxInt || uint64(len(r.Literal.Pattern)-1) > uint64(math.MaxInt)-size {
		return errors.Join(core.ErrLineIOContract, err)
	}
	return nil
}

type ReplacementCount uint64

// ReplaceStreamObservation is published only after bare native EOF and all
// destination writes succeed. Prefix output is provisional on refusal.
type ReplaceStreamObservation struct {
	InputBytes   core.ByteLength
	OutputBytes  core.ByteLength
	Replacements ReplacementCount
}

func (r ReplaceStreamObservation) Validate() error {
	_, inputErr := r.InputBytes.Int64()
	_, outputErr := r.OutputBytes.Int64()
	if inputErr != nil || outputErr != nil {
		return errors.Join(core.ErrLineIOContract, inputErr, outputErr)
	}
	return nil
}

// ReplaceStream implements bytes.ReplaceAll's exact, leftmost nonoverlapping
// meaning across fixed read windows. It retains one window and at most
// len(Pattern)-1 undecided bytes. Memory depends on working memory and the
// supplied pattern, not source length, record length or match count. The caller
// owns interruption and cleanup of the borrowed native capabilities.
func ReplaceStream(ctx context.Context, request ReplaceStreamRequest) (ReplaceStreamObservation, error) {
	if err := errors.Join(contextstate.Validate(ctx), request.Validate()); err != nil {
		return ReplaceStreamObservation{}, err
	}
	size, _ := request.BufferBytes.Uint64()
	window := make([]byte, int(size)+len(request.Literal.Pattern)-1)
	source := bufio.NewReaderSize(checkedReader{source: request.Source}, int(size))
	var inputBytes, outputBytes int64
	var matches ReplacementCount
	pending := 0
	write := func(data []byte) error {
		if err := contextstate.Validate(ctx); err != nil {
			return err
		}
		if len(data) == 0 {
			return nil
		}
		if int64(len(data)) > math.MaxInt64-outputBytes {
			return core.ErrLineIOContract
		}
		count, err := request.Destination.Write(data)
		if count < 0 || count > len(data) {
			return errors.Join(core.ErrLineIOContract, err)
		}
		outputBytes += int64(count)
		if count != len(data) && err == nil {
			err = io.ErrShortWrite
		}
		return errors.Join(contextstate.Validate(ctx), err)
	}
	for {
		if err := contextstate.Validate(ctx); err != nil {
			return ReplaceStreamObservation{}, err
		}
		count, readErr := source.Read(window[pending : pending+int(size)])
		if int64(count) > math.MaxInt64-inputBytes {
			return ReplaceStreamObservation{}, core.ErrLineIOContract
		}
		inputBytes += int64(count)
		if err := contextstate.Validate(ctx); err != nil {
			return ReplaceStreamObservation{}, errors.Join(err, readErr)
		}
		available := window[:pending+count]
		safe := len(available) - (len(request.Literal.Pattern) - 1)
		if safe < 0 {
			safe = 0
		}
		if readErr == io.EOF {
			safe = len(available)
		}
		for safe > 0 {
			match := bytes.Index(available, request.Literal.Pattern)
			if match < 0 || match >= safe {
				if err := write(available[:safe]); err != nil {
					return ReplaceStreamObservation{}, err
				}
				available = available[safe:]
				break
			}
			if matches == ReplacementCount(math.MaxUint64) {
				return ReplaceStreamObservation{}, core.ErrLineIOContract
			}
			if err := write(available[:match]); err != nil {
				return ReplaceStreamObservation{}, err
			}
			if err := write(request.Literal.Replacement); err != nil {
				return ReplaceStreamObservation{}, err
			}
			matches++
			available = available[match+len(request.Literal.Pattern):]
			safe = len(available) - (len(request.Literal.Pattern) - 1)
			if safe < 0 {
				safe = 0
			}
			if readErr == io.EOF {
				safe = len(available)
			}
		}
		pending = copy(window, available)
		if readErr != nil {
			if readErr != io.EOF {
				return ReplaceStreamObservation{}, errors.Join(core.ErrLineIOScan, readErr)
			}
			inputExtent, err := core.NewByteLength(uint64(inputBytes))
			if err != nil {
				return ReplaceStreamObservation{}, err
			}
			outputExtent, err := core.NewByteLength(uint64(outputBytes))
			if err != nil {
				return ReplaceStreamObservation{}, err
			}
			return ReplaceStreamObservation{InputBytes: inputExtent, OutputBytes: outputExtent, Replacements: matches}, nil
		}
	}
}
