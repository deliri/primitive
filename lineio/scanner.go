package lineio

import (
	"bufio"
	"errors"
	"io"

	"github.com/deliri/primitive/v2026/core"
)

const (
	// MaximumLineBytes is Lineio's fixed per-line and initial-allocation
	// ceiling.
	MaximumLineBytes               uint64 = 16 << 20
	scanLinesMaximumDelimiterBytes        = 2
)

// BufferPolicy bounds one scanner's initial allocation and every emitted line.
type BufferPolicy struct {
	InitialBytes     core.ByteCount
	MaximumLineBytes core.ByteCount
}

// Validate rejects unset, contradictory, or unrepresentable scanner bounds.
func (p BufferPolicy) Validate() error {
	if err := p.InitialBytes.Validate(); err != nil {
		return errors.Join(core.ErrLineIOContract, err)
	}
	if err := p.MaximumLineBytes.Validate(); err != nil {
		return errors.Join(core.ErrLineIOContract, err)
	}
	initial, err := p.InitialBytes.Uint64()
	if err != nil {
		return errors.Join(core.ErrLineIOContract, err)
	}
	maximum, err := p.MaximumLineBytes.Uint64()
	if err != nil {
		return errors.Join(core.ErrLineIOContract, err)
	}
	if initial > maximum {
		return core.ErrLineIOContract
	}
	if maximum > MaximumLineBytes {
		return core.ErrLineIOContract
	}
	return nil
}

// Request supplies one source and the complete buffer policy for its scanner.
type Request struct {
	Source io.Reader
	Buffer BufferPolicy
}

// Validate rejects an unusable reader or invalid buffer policy.
func (r Request) Validate() error {
	if core.ReaderIsNil(r.Source) {
		return core.ErrLineIOContract
	}
	return r.Buffer.Validate()
}

// Scanner is one bounded bufio.ScanLines stream. It owns no goroutine, pool,
// handle, or cleanup protocol; the caller retains ownership of the reader.
type Scanner struct {
	scanner *bufio.Scanner
	err     error
}

// MaximumLineByteCount returns Lineio's fixed per-line and initial-allocation
// ceiling. The ceiling is independent of the running Go target.
func MaximumLineByteCount() (core.ByteCount, error) {
	return core.NewByteCount(MaximumLineBytes)
}

// New validates one request, allocates only its declared initial buffer, and
// binds Go's scanner to the exact declared line ceiling.
func New(request Request) (*Scanner, error) {
	if err := request.Validate(); err != nil {
		return nil, err
	}
	initial, maximum, err := request.Buffer.nativeBounds()
	if err != nil {
		return nil, err
	}
	scanner := bufio.NewScanner(request.Source)
	scanner.Buffer(make([]byte, initial), maximum+scanLinesMaximumDelimiterBytes)
	scanner.Split(boundedScanLines(maximum))
	return &Scanner{scanner: scanner}, nil
}

// Validate rejects a nil or uninitialized scanner.
func (s *Scanner) Validate() error {
	if s == nil || s.scanner == nil {
		return core.ErrLineIOContract
	}
	return nil
}

// Scan advances to the next bounded line.
func (s *Scanner) Scan() bool {
	if s.Validate() != nil || s.err != nil {
		return false
	}
	if s.scanner.Scan() {
		return true
	}
	s.err = classifyScanError(s.scanner.Err())
	return false
}

// Bytes returns the current line view. As with bufio.Scanner, the view may be
// overwritten by the next call to Scan.
func (s *Scanner) Bytes() []byte {
	if s.Validate() != nil {
		return nil
	}
	return s.scanner.Bytes()
}

// Err returns the typed scan failure, the native reader or bufio identity it
// wraps, or nil after clean exhaustion.
func (s *Scanner) Err() error {
	if err := s.Validate(); err != nil {
		return err
	}
	return s.err
}

func (p BufferPolicy) nativeBounds() (int, int, error) {
	if err := p.Validate(); err != nil {
		return 0, 0, err
	}
	initial, err := p.InitialBytes.Uint64()
	if err != nil {
		return 0, 0, errors.Join(core.ErrLineIOContract, err)
	}
	maximum, err := p.MaximumLineBytes.Uint64()
	if err != nil {
		return 0, 0, errors.Join(core.ErrLineIOContract, err)
	}
	initialBytes := int(initial)     // #nosec G115 -- BufferPolicy.Validate caps the value at Lineio's 16 MiB ceiling.
	maximumLineBytes := int(maximum) // #nosec G115 -- BufferPolicy.Validate caps the value at Lineio's 16 MiB ceiling.
	return initialBytes, maximumLineBytes, nil
}

func boundedScanLines(maximum int) bufio.SplitFunc {
	return func(data []byte, atEOF bool) (int, []byte, error) {
		advance, token, err := bufio.ScanLines(data, atEOF)
		if err != nil {
			return 0, nil, classifyScanError(err)
		}
		if token != nil {
			if len(token) > maximum {
				return 0, nil, lineTooLongError()
			}
			return advance, token, nil
		}
		if linePrefixExceedsMaximum(data, maximum) {
			return 0, nil, lineTooLongError()
		}
		return advance, nil, nil
	}
}

func linePrefixExceedsMaximum(data []byte, maximum int) bool {
	if len(data) <= maximum {
		return false
	}
	return len(data) != maximum+1 || data[maximum] != '\r'
}

func lineTooLongError() error {
	return errors.Join(core.ErrLineIOScan, bufio.ErrTooLong)
}

func classifyScanError(err error) error {
	if err == nil || errors.Is(err, core.ErrLineIOScan) {
		return err
	}
	return errors.Join(core.ErrLineIOScan, err)
}
