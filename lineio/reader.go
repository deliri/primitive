package lineio

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"math"

	"github.com/deliri/primitive/v2026/core"
)

// Delimiter is the LF framing byte consumed by Go's ReadSlice.
const Delimiter byte = '\n'

// Request configures memory, not the amount of input admitted. Go may raise a
// very small BufferBytes request to its own minimum reader buffer size.
type Request struct {
	Source      io.Reader
	BufferBytes core.ByteCount
}

// Validate checks reader identity and native buffer-size representability.
// There is no package-imposed allocation, line, or total-stream ceiling.
func (r Request) Validate() error {
	_, err := r.bufferSize()
	return err
}

func (r Request) bufferSize() (int, error) {
	if core.ReaderIsNil(r.Source) {
		return 0, core.ErrLineIOContract
	}
	size, err := r.BufferBytes.Uint64()
	if err != nil {
		return 0, errors.Join(core.ErrLineIOContract, err)
	}
	if size > math.MaxInt {
		return 0, core.ErrLineIOContract
	}
	return int(size), nil // #nosec G115 -- native representability checked above.
}

// Fragment is a borrowed view of the next exact source bytes, including LF if
// present. More means the buffer filled before LF: continue the same line.
// A completing More=false fragment still belongs to the preceding More=true
// fragments of the same line. A completing fragment containing only LF does not
// introduce an empty line after that prefix. Consume all fragments in order.
// When More is false, inspect the accompanying error to distinguish a complete
// LF-terminated line, clean EOF, and an interrupted partial line.
// The bytes may be overwritten by the next ReadFragment call.
type Fragment struct {
	Bytes []byte
	More  bool
}

// Validate checks fragment framing only. Its zero value is neutral, and a
// fragment's length is never used as an input acceptance quota.
func (f Fragment) Validate() error {
	if f.More && len(f.Bytes) == 0 {
		return core.ErrLineIOContract
	}
	if index := bytes.IndexByte(f.Bytes, Delimiter); index >= 0 && (f.More || index != len(f.Bytes)-1) {
		return core.ErrLineIOContract
	}
	return nil
}

// Reader owns one fixed Go buffer. The caller retains source lifetime and
// cleanup ownership. It is for sequential use, just like bufio.Reader.
type Reader struct {
	buffer   *bufio.Reader
	terminal error
}

// New allocates one declared buffer without reading the source. Buffers never
// grow to fit a line. A reader-count guard also ensures an existing bufio.Reader
// is treated as the source, rather than reusing its independently owned buffer.
func New(request Request) (*Reader, error) {
	size, err := request.bufferSize()
	if err != nil {
		return nil, err
	}
	return &Reader{buffer: bufio.NewReaderSize(checkedReader{source: request.Source}, size)}, nil
}

// Validate rejects nil or uninitialized readers.
func (r *Reader) Validate() error {
	if r == nil || r.buffer == nil {
		return core.ErrLineIOContract
	}
	return nil
}

// Capacity reports the actual fixed buffer capacity selected by Go.
func (r *Reader) Capacity() (core.ByteCount, error) {
	if err := r.Validate(); err != nil {
		return core.ByteCount{}, err
	}
	size := r.buffer.Size()
	if size < 0 {
		return core.ByteCount{}, core.ErrLineIOContract
	}
	return core.NewByteCount(uint64(size))
}

// ReadFragment returns exact bytes before any accompanying error, like io.Reader.
// A full buffer returns More=true and nil error. Clean EOF is io.EOF, including
// when it accompanies a final unterminated fragment. Other errors retain both
// the native identity and ErrLineIOScan. A terminal error never triggers a retry.
func (r *Reader) ReadFragment() (Fragment, error) {
	if err := r.Validate(); err != nil {
		return Fragment{}, err
	}
	if r.terminal != nil {
		return Fragment{}, r.terminal
	}
	data, err := r.buffer.ReadSlice(Delimiter)
	// witness:waiver doctrine/error/sentinel_compare -- Lineio owner; Go 1.27 ReadSlice uses this exact sentinel for its own full window. Source failures are wrapped by checkedReader. Recheck on the next Go upgrade.
	if err == bufio.ErrBufferFull {
		return Fragment{Bytes: data, More: true}, nil
	}
	// witness:waiver doctrine/error/sentinel_compare -- Lineio owner; io.Reader requires exact io.EOF for clean completion. A wrapped EOF remains a source failure. Recheck on the next Go upgrade.
	if err != nil && err != io.EOF && !errors.Is(err, core.ErrLineIOScan) {
		err = errors.Join(core.ErrLineIOScan, err)
	}
	r.terminal = err
	return Fragment{Bytes: data}, err
}

// checkedReader preserves Go's read-count contract before bufio uses the count.
type checkedReader struct{ source io.Reader }

func (r checkedReader) Read(p []byte) (int, error) {
	n, err := r.source.Read(p)
	if n < 0 || n > len(p) {
		return 0, errors.Join(bufio.ErrBadReadCount, err)
	}
	if errors.Is(err, bufio.ErrBufferFull) {
		err = errors.Join(core.ErrLineIOScan, err)
	}
	return n, err
}
