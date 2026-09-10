package lineio_test

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"slices"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lineio"
)

const streamBufferFixture uint64 = 64

type hostileReaderError struct{}

func (hostileReaderError) Error() string { return "lineio hostile reader failure" }

type observedReader struct {
	io.Reader
	calls  int
	bytes  int
	closed bool
}

func (r *observedReader) Read(p []byte) (int, error) {
	r.calls++
	n, err := r.Reader.Read(p)
	if n > 0 {
		r.bytes += n
	}
	return n, err
}
func (r *observedReader) Close() error { r.closed = true; return nil }

type terminalReader struct {
	*strings.Reader
	cause error
	chunk int
}

func (r *terminalReader) Read(p []byte) (int, error) {
	if r.chunk > 0 && len(p) > r.chunk {
		p = p[:r.chunk]
	}
	n, err := r.Reader.Read(p)
	if r.Len() == 0 {
		return n, r.cause
	}
	return n, err
}

type invalidCountReader int

func (r invalidCountReader) Read(p []byte) (int, error) {
	if r < 0 {
		return -1, nil
	}
	return len(p) + 1, nil
}

type stalledReader struct {
	remaining int
	source    io.Reader
}

func (r *stalledReader) Read(p []byte) (int, error) {
	if r.remaining > 0 {
		r.remaining--
		return 0, nil
	}
	return r.source.Read(p)
}

type fillReader byte

func (r fillReader) Read(p []byte) (int, error) {
	for index := range p {
		p[index] = byte(r)
	}
	return len(p), nil
}
func mustByteCount(t testing.TB, value uint64) core.ByteCount {
	t.Helper()
	count, err := core.NewByteCount(value)
	if err != nil {
		t.Fatalf("NewByteCount(%d) = %v, want nil", value, err)
	}
	return count
}

func TestReaderIngressLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name          string
		source        func() io.Reader
		body          string
		wantErr       error
		wantScanError bool
	}{
		{name: "empty source does not invent a fragment"},
		{name: "one LF remains one byte", body: "\n"},
		{name: "empty CRLF retains both bytes", body: "\r\n"},
		{name: "final CR is not normalized", body: "\r"},
		{name: "unterminated final bytes remain exact", body: "tail"},
		{name: "mixed line endings retain every byte", body: "a\r\nb\nc\r"},
		{name: "empty middle and trailing lines remain present", body: "a\n\n\n"},
		{name: "opaque invalid UTF8 NUL and whitespace remain bytes", body: "\t\xff\x00 é \n"},
		{name: "one below buffer with LF", body: strings.Repeat("x", int(streamBufferFixture)-2) + "\n"},
		{name: "exact buffer with LF", body: strings.Repeat("x", int(streamBufferFixture)-1) + "\n"},
		{name: "one beyond buffer with LF continues", body: strings.Repeat("x", int(streamBufferFixture)) + "\n"},
		{name: "one below buffer at EOF", body: strings.Repeat("x", int(streamBufferFixture)-1)},
		{name: "exact buffer at EOF", body: strings.Repeat("x", int(streamBufferFixture))},
		{name: "one beyond buffer at EOF continues", body: strings.Repeat("x", int(streamBufferFixture)+1)},
		{name: "CRLF straddling buffer remains exact", body: strings.Repeat("x", int(streamBufferFixture)-1) + "\r\n"},
		{name: "CR followed by data across buffer is retained", body: strings.Repeat("x", int(streamBufferFixture)-1) + "\ry\n"},
		{name: "line greatly exceeding buffer continues", body: strings.Repeat("x", 4096) + "\n"},
		{name: "complete prefix then long final line remains exact", body: "kept\n" + strings.Repeat("x", 4096)},
		{name: "one-byte reads preserve CRLF", body: "a\r\nlast", source: func() io.Reader { return iotest.OneByteReader(strings.NewReader("a\r\nlast")) }},
		{name: "half reads preserve line ordering", body: "one\ntwo\nlast", source: func() io.Reader { return iotest.HalfReader(strings.NewReader("one\ntwo\nlast")) }},
		{name: "data and EOF retain the final bytes", body: "kept\nlast", source: func() io.Reader { return iotest.DataErrReader(strings.NewReader("kept\nlast")) }},
		{name: "data and native failure retain bytes and cause", body: "kept\nlast", source: func() io.Reader {
			return &terminalReader{Reader: strings.NewReader("kept\nlast"), cause: hostileReaderError{}}
		}, wantErr: hostileReaderError{}, wantScanError: true},
		{name: "immediate cancellation stays cancellation", source: func() io.Reader { return iotest.ErrReader(context.Canceled) }, wantErr: context.Canceled, wantScanError: true},
		{name: "deadline with partial data retains both", body: "kept", source: func() io.Reader {
			return &terminalReader{Reader: strings.NewReader("kept"), cause: context.DeadlineExceeded}
		}, wantErr: context.DeadlineExceeded, wantScanError: true},
		{name: "wrapped EOF remains a failure", body: "kept", source: func() io.Reader {
			return &terminalReader{Reader: strings.NewReader("kept"), cause: fmt.Errorf("source: %w", io.EOF)}
		}, wantErr: io.EOF, wantScanError: true},
		{name: "negative read count refuses without panic", source: func() io.Reader { return invalidCountReader(-1) }, wantErr: bufio.ErrBadReadCount, wantScanError: true},
		{name: "read count beyond destination refuses without panic", source: func() io.Reader { return invalidCountReader(1) }, wantErr: bufio.ErrBadReadCount, wantScanError: true},
		{name: "nonprogress reader is refused by Go", source: func() io.Reader {
			return &stalledReader{remaining: 1024, source: iotest.ErrReader(hostileReaderError{})}
		}, wantErr: io.ErrNoProgress, wantScanError: true},
		{name: "temporary empty read resumes without data loss", body: "kept\n", source: func() io.Reader { return &stalledReader{remaining: 1, source: strings.NewReader("kept\n")} }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var source io.Reader = strings.NewReader(tc.body)
			if tc.source != nil {
				source = tc.source()
			}
			observed := &observedReader{Reader: source}
			request := lineio.Request{Source: observed, BufferBytes: mustByteCount(t, streamBufferFixture)}
			if err := request.Validate(); err != nil {
				t.Fatalf("Request.Validate() = %v, want nil", err)
			}
			reader, err := lineio.New(request)
			if err != nil {
				t.Fatalf("New() = %v, want nil", err)
			}
			capacity, err := reader.Capacity()
			if err != nil || capacity != request.BufferBytes || observed.calls != 0 {
				t.Fatalf("capacity/read effects = (%v,%v,%d), want (%v,nil,0)", capacity, err, observed.calls, request.BufferBytes)
			}
			var got []byte
			var terminal error
			for {
				fragment, readErr := reader.ReadFragment()
				if err := fragment.Validate(); err != nil {
					t.Fatalf("Fragment.Validate() = %v, want nil", err)
				}
				if fragment.More && (readErr != nil || len(fragment.Bytes) != int(streamBufferFixture)) {
					t.Fatalf("continuation = (%d bytes,%v), want full buffer and nil", len(fragment.Bytes), readErr)
				}
				if readErr == nil && !fragment.More && (len(fragment.Bytes) == 0 || fragment.Bytes[len(fragment.Bytes)-1] != lineio.Delimiter) {
					t.Fatalf("complete fragment = %q, want final LF", fragment.Bytes)
				}
				got = append(got, fragment.Bytes...)
				if len(got) > len(tc.body) {
					t.Fatalf("produced %d bytes, want at most %d", len(got), len(tc.body))
				}
				if readErr != nil {
					terminal = readErr
					break
				}
			}
			if !bytes.Equal(got, []byte(tc.body)) {
				t.Fatalf("bytes = %q, want %q", got, tc.body)
			}
			wantErr := tc.wantErr
			if wantErr == nil {
				wantErr = io.EOF
			}
			if !errors.Is(terminal, wantErr) || errors.Is(terminal, core.ErrLineIOScan) != tc.wantScanError {
				t.Fatalf("terminal = %v, want %v, scan identity %t", terminal, wantErr, tc.wantScanError)
			}
			if tc.wantErr == (hostileReaderError{}) {
				if _, ok := errors.AsType[hostileReaderError](terminal); !ok {
					t.Fatalf("terminal = %v, want typed native cause", terminal)
				}
			}
			calls := observed.calls
			for range 3 {
				fragment, err := reader.ReadFragment()
				if len(fragment.Bytes) != 0 || fragment.More || err != terminal || observed.calls != calls {
					t.Fatalf("terminal retry = (%v,%v,%d reads), want zero fragment, retained error, %d reads", fragment, err, observed.calls, calls)
				}
			}
			if observed.closed || observed.calls == 0 {
				t.Fatalf("reader effects = (%d reads, closed %t), want reads and caller-owned open reader", observed.calls, observed.closed)
			}
		})
	}
}

func TestRequestMemoryConfigurationLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		size      uint64
		source    func() io.Reader
		wantErr   error
		construct bool
	}{
		{name: "unset buffer is invalid", wantErr: core.ErrLineIOContract, construct: true},
		{name: "nil source is invalid", size: 64, source: func() io.Reader { return nil }, wantErr: core.ErrLineIOContract, construct: true},
		{name: "typed nil source is invalid", size: 64, source: func() io.Reader { var source *bytes.Buffer; return source }, wantErr: core.ErrLineIOContract, construct: true},
		{name: "small buffer uses Go minimum without input quota", size: 1, construct: true},
		{name: "ordinary fixed buffer accepted", size: 64, construct: true},
		{name: "allocation beyond retired 16MiB cap is caller owned", size: (16 << 20) + 1, construct: true},
		{name: "native buffer extent is representable without allocating it", size: math.MaxInt},
		{name: "native extent plus one is not a buffer size", size: uint64(math.MaxInt) + 1, wantErr: core.ErrLineIOContract, construct: true},
		{name: "unsigned extreme cannot become native buffer size", size: math.MaxUint64, wantErr: core.ErrLineIOContract, construct: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			source := &observedReader{Reader: strings.NewReader("")}
			request := lineio.Request{Source: source}
			if tc.size > 0 {
				request.BufferBytes = mustByteCount(t, tc.size)
			}
			if tc.source != nil {
				request.Source = tc.source()
			}
			if err := request.Validate(); !errors.Is(err, tc.wantErr) {
				t.Fatalf("Validate() = %v, want %v", err, tc.wantErr)
			}
			if !tc.construct {
				return
			}
			reader, err := lineio.New(request)
			if !errors.Is(err, tc.wantErr) || (reader == nil) != (tc.wantErr != nil) || source.calls != 0 {
				t.Fatalf("New() = (%v,%v,%d reads), want rejected %t, %v, no reads", reader, err, source.calls, tc.wantErr != nil, tc.wantErr)
			}
			if reader != nil {
				capacity, err := reader.Capacity()
				if err != nil {
					t.Fatalf("Capacity() = %v, want nil", err)
				}
				value, err := capacity.Uint64()
				if err != nil || value < tc.size {
					t.Fatalf("capacity = (%d,%v), want at least requested %d", value, err, tc.size)
				}
			}
		})
	}
}

func TestReaderStateExhaustive(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		construct bool
		zero      bool
		wantErr   error
	}{
		{name: "nil reader refuses every operation", wantErr: core.ErrLineIOContract},
		{name: "zero reader refuses every operation", zero: true, wantErr: core.ErrLineIOContract},
		{name: "constructed empty reader reaches clean EOF", construct: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var reader *lineio.Reader
			if tc.zero {
				reader = &lineio.Reader{}
			}
			if tc.construct {
				var err error
				reader, err = lineio.New(lineio.Request{Source: strings.NewReader(""), BufferBytes: mustByteCount(t, 64)})
				if err != nil {
					t.Fatalf("New() = %v, want nil", err)
				}
			}
			if err := reader.Validate(); !errors.Is(err, tc.wantErr) {
				t.Fatalf("Validate() = %v, want %v", err, tc.wantErr)
			}
			capacity, err := reader.Capacity()
			if !errors.Is(err, tc.wantErr) || (capacity == core.ByteCount{}) != (tc.wantErr != nil) {
				t.Fatalf("Capacity() = (%v,%v), want zero %t, %v", capacity, err, tc.wantErr != nil, tc.wantErr)
			}
			fragment, err := reader.ReadFragment()
			wantErr := tc.wantErr
			if tc.construct {
				wantErr = io.EOF
			}
			if !errors.Is(err, wantErr) || len(fragment.Bytes) != 0 || fragment.More {
				t.Fatalf("ReadFragment() = (%v,%v), want neutral fragment and %v", fragment, err, wantErr)
			}
		})
	}
}

func TestFragmentFramingLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		fragment lineio.Fragment
		wantErr  error
	}{
		{name: "zero fragment is neutral"},
		{name: "opaque continuation is valid", fragment: lineio.Fragment{Bytes: []byte("x\r\x00"), More: true}},
		{name: "LF completion retains delimiter", fragment: lineio.Fragment{Bytes: []byte("x\n")}},
		{name: "unterminated terminal bytes are valid", fragment: lineio.Fragment{Bytes: []byte("x\r")}},
		{name: "empty continuation is invalid", fragment: lineio.Fragment{More: true}, wantErr: core.ErrLineIOContract},
		{name: "LF cannot claim continuation", fragment: lineio.Fragment{Bytes: []byte("\n"), More: true}, wantErr: core.ErrLineIOContract},
		{name: "bytes after LF cannot share a fragment", fragment: lineio.Fragment{Bytes: []byte("a\nb")}, wantErr: core.ErrLineIOContract},
		{name: "multiple line ends cannot share a fragment", fragment: lineio.Fragment{Bytes: []byte("\n\n")}, wantErr: core.ErrLineIOContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if err := tc.fragment.Validate(); !errors.Is(err, tc.wantErr) {
				t.Fatalf("Fragment.Validate() = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestReaderFixedMemoryBeyondLineExtents(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		buffer uint64
		extent int64
	}{
		{name: "large unterminated line with small buffer", buffer: 64, extent: 1 << 20},
		{name: "line beyond former eager cap with fixed buffer", buffer: 64 << 10, extent: (16 << 20) + 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			source := &observedReader{Reader: io.LimitReader(fillReader('x'), tc.extent)}
			reader, err := lineio.New(lineio.Request{Source: source, BufferBytes: mustByteCount(t, tc.buffer)})
			if err != nil {
				t.Fatalf("New() = %v, want nil", err)
			}
			var total int64
			var fragments int
			for {
				fragment, err := reader.ReadFragment()
				if bytes.Count(fragment.Bytes, []byte{'x'}) != len(fragment.Bytes) {
					t.Fatalf("fragment = %q, want only source bytes", fragment.Bytes)
				}
				total += int64(len(fragment.Bytes))
				if len(fragment.Bytes) > 0 {
					fragments++
				}
				capacity, capacityErr := reader.Capacity()
				if capacityErr != nil || capacity != mustByteCount(t, tc.buffer) || len(fragment.Bytes) > int(tc.buffer) {
					t.Fatalf("buffer = (%v,%v,%d bytes), want fixed %d", capacity, capacityErr, len(fragment.Bytes), tc.buffer)
				}
				if total > tc.extent {
					t.Fatalf("bytes = %d, want at most %d", total, tc.extent)
				}
				if err != nil {
					if !errors.Is(err, io.EOF) || errors.Is(err, core.ErrLineIOScan) {
						t.Fatalf("terminal = %v, want EOF without extent refusal", err)
					}
					break
				}
			}
			if total != tc.extent || fragments < 2 || int64(source.bytes) != tc.extent {
				t.Fatalf("stream = (%d bytes,%d fragments,%d source bytes), want %d bytes in fragments", total, fragments, source.bytes, tc.extent)
			}
		})
	}
}

// No shared mutable fixture: every traversal owns its source and collection.
func TestReaderDelimiterPermutationLayerTriad(t *testing.T) {
	t.Parallel()
	alphabet := []byte{'x', '\r', '\n', 0xff}
	inputs := [][]byte{nil}
	level := [][]byte{nil}
	for range 4 {
		var next [][]byte
		for _, prefix := range level {
			for _, symbol := range alphabet {
				next = append(next, append(bytes.Clone(prefix), symbol))
			}
		}
		inputs = append(inputs, next...)
		level = next
	}
	for _, input := range inputs {
		t.Run(fmt.Sprintf("bytes_%x", input), func(t *testing.T) {
			t.Parallel()
			for _, chunk := range []int{1, 2, 4} {
				reader, err := lineio.New(lineio.Request{Source: &terminalReader{Reader: strings.NewReader(string(input)), cause: io.EOF, chunk: chunk}, BufferBytes: mustByteCount(t, 1)})
				if err != nil {
					t.Fatalf("New() = %v, want nil", err)
				}
				var got []byte
				for {
					fragment, err := reader.ReadFragment()
					got = append(got, fragment.Bytes...)
					if err != nil {
						if !errors.Is(err, io.EOF) || errors.Is(err, core.ErrLineIOScan) {
							t.Fatalf("terminal = %v, want EOF", err)
						}
						break
					}
				}
				if !slices.Equal(got, input) {
					t.Fatalf("chunk %d bytes = %q, want %q", chunk, got, input)
				}
			}
		})
	}
}

func TestReaderOwnsItsBufferLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name         string
		sourceBuffer int
		requested    uint64
	}{
		{name: "larger caller buffer cannot replace smaller owned window", sourceBuffer: 4096, requested: streamBufferFixture},
		{name: "equal caller buffer remains independently owned", sourceBuffer: int(streamBufferFixture), requested: streamBufferFixture},
		{name: "smaller caller buffer does not shrink requested window", sourceBuffer: 16, requested: streamBufferFixture},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			body := strings.Repeat("x", 4096) + "\r\n"
			source := bufio.NewReaderSize(strings.NewReader(body), tc.sourceBuffer)
			reader, err := lineio.New(lineio.Request{Source: source, BufferBytes: mustByteCount(t, tc.requested)})
			if err != nil {
				t.Fatalf("New() = %v, want nil", err)
			}
			capacity, err := reader.Capacity()
			if err != nil || capacity != mustByteCount(t, tc.requested) {
				t.Fatalf("Capacity() = (%v,%v), want requested %d independent of source %d", capacity, err, tc.requested, tc.sourceBuffer)
			}
			var total int
			for {
				fragment, readErr := reader.ReadFragment()
				if len(fragment.Bytes) > int(tc.requested) || total+len(fragment.Bytes) > len(body) {
					t.Fatalf("fragment = %d bytes at %d, want window %d within body %d", len(fragment.Bytes), total, tc.requested, len(body))
				}
				if !bytes.Equal(fragment.Bytes, []byte(body[total:total+len(fragment.Bytes)])) {
					t.Fatalf("fragment at %d = %q, want exact source", total, fragment.Bytes)
				}
				total += len(fragment.Bytes)
				if readErr != nil {
					if !errors.Is(readErr, io.EOF) || total != len(body) {
						t.Fatalf("terminal = (%v,%d bytes), want EOF and %d", readErr, total, len(body))
					}
					break
				}
			}
		})
	}
}

func TestReaderSourceBufferErrorLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		body  string
		cause error
	}{
		{name: "immediate source buffer failure is terminal", cause: bufio.ErrBufferFull},
		{name: "partial data with source buffer failure is terminal", body: "partial", cause: bufio.ErrBufferFull},
		{name: "full window with source buffer failure is terminal", body: strings.Repeat("x", int(streamBufferFixture)), cause: bufio.ErrBufferFull},
		{name: "wrapped source buffer failure preserves cause", body: "partial", cause: fmt.Errorf("source: %w", bufio.ErrBufferFull)},
		{name: "ordinary source failure stays terminal", body: "partial", cause: hostileReaderError{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			source := &observedReader{Reader: &terminalReader{Reader: strings.NewReader(tc.body), cause: tc.cause}}
			reader, err := lineio.New(lineio.Request{Source: source, BufferBytes: mustByteCount(t, streamBufferFixture)})
			if err != nil {
				t.Fatalf("New() = %v, want nil", err)
			}
			fragment, err := reader.ReadFragment()
			if !errors.Is(err, tc.cause) || !errors.Is(err, core.ErrLineIOScan) || fragment.More || !bytes.Equal(fragment.Bytes, []byte(tc.body)) {
				t.Fatalf("source failure = (%q,more %t,%v), want (%q,false,%v) with scan identity", fragment.Bytes, fragment.More, err, tc.body, tc.cause)
			}
			calls := source.calls
			fragment, err = reader.ReadFragment()
			if !errors.Is(err, tc.cause) || len(fragment.Bytes) != 0 || fragment.More || source.calls != calls {
				t.Fatalf("terminal replay = (%v,%v,%d reads), want neutral and %v without read beyond %d", fragment, err, source.calls, tc.cause, calls)
			}
		})
	}
}

func TestReaderLineCompletionLayerTriad(t *testing.T) {
	t.Parallel()
	prefix := strings.Repeat("x", int(streamBufferFixture)-1)
	cases := []struct {
		name   string
		source string
		want   []string
	}{
		{name: "empty source invents no line"},
		{name: "bare LF is a real empty line", source: "\n", want: []string{"\n"}},
		{name: "LF alone completes a full preceding window", source: prefix + "x\n", want: []string{prefix + "x\n"}},
		{name: "LF alone completes a CR aligned window", source: prefix + "\r\n", want: []string{prefix + "\r\n"}},
		{name: "next empty line survives after CR aligned completion", source: prefix + "\r\n\n", want: []string{prefix + "\r\n", "\n"}},
		{name: "exact window LF completion does not invent an EOF line", source: prefix + "\n", want: []string{prefix + "\n"}},
		{name: "exact unterminated window completes at EOF", source: prefix + "x", want: []string{prefix + "x"}},
		{name: "partial final window completes the preceding line", source: prefix + "xy", want: []string{prefix + "xy"}},
		{name: "CR outside CRLF remains ordinary final data", source: prefix + "\ry\nlast\r", want: []string{prefix + "\ry\n", "last\r"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			reader, err := lineio.New(lineio.Request{Source: strings.NewReader(tc.source), BufferBytes: mustByteCount(t, streamBufferFixture)})
			if err != nil {
				t.Fatalf("New() = %v, want nil", err)
			}
			var lines []string
			var current []byte
			for {
				fragment, readErr := reader.ReadFragment()
				if err := fragment.Validate(); err != nil {
					t.Fatalf("fragment = %v, want valid framing", err)
				}
				current = append(current, fragment.Bytes...)
				if !fragment.More && len(current) > 0 {
					lines = append(lines, string(current))
					current = current[:0]
				}
				if readErr != nil {
					if !errors.Is(readErr, io.EOF) || errors.Is(readErr, core.ErrLineIOScan) {
						t.Fatalf("terminal = %v, want clean EOF", readErr)
					}
					break
				}
			}
			if len(current) != 0 || !slices.Equal(lines, tc.want) {
				t.Fatalf("lines = %q, unfinished %q, want %q with no unfinished prefix", lines, current, tc.want)
			}
		})
	}
}
