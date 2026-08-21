package lineio_test

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"slices"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lineio"
)

const (
	hostileInitialBytes     = 4
	hostileMaximumLineBytes = 64
)

type hostileReaderError struct{}

func (hostileReaderError) Error() string { return "lineio hostile reader failure" }

type hostileCaseClass uint8

const (
	hostileCaseUnknown hostileCaseClass = iota
	hostileCasePositive
	hostileCaseNegative
	hostileCaseBoundary
	hostileCaseLimit
)

func (c hostileCaseClass) Validate() error {
	if c <= hostileCaseUnknown || c >= hostileCaseLimit {
		return core.ErrPrimitiveContract
	}
	return nil
}

type scannerHostileCase struct {
	request       func(*testing.T) lineio.Request
	wantNativeErr error
	wantNewErr    error
	wantScanErr   error
	name          string
	wantLines     []string
	class         hostileCaseClass
}

func TestScannerIngressLayerTriadHostileTable(t *testing.T) {
	t.Parallel()

	maximumSupported, err := lineio.MaximumLineByteCount()
	if err != nil {
		t.Fatalf("MaximumLineByteCount() error = %v, want nil", err)
	}
	maximumSupportedValue, err := maximumSupported.Uint64()
	if err != nil {
		t.Fatalf("MaximumLineByteCount().Uint64() error = %v, want nil", err)
	}
	if maximumSupportedValue != lineio.MaximumLineBytes {
		t.Fatalf("MaximumLineByteCount().Uint64() = %d, want MaximumLineBytes %d", maximumSupportedValue, lineio.MaximumLineBytes)
	}
	maximumSupportedMinusOne := mustByteCount(t, maximumSupportedValue-1)
	maximumSupportedPlusOne := mustByteCount(t, maximumSupportedValue+1)

	cases := []scannerHostileCase{
		{name: "positive single LF line is emitted", class: hostileCasePositive, request: scannerRequest("alpha\n", hostileInitialBytes, hostileMaximumLineBytes), wantLines: []string{"alpha"}},
		{name: "positive two LF lines preserve order", class: hostileCasePositive, request: scannerRequest("alpha\nbeta\n", hostileInitialBytes, hostileMaximumLineBytes), wantLines: []string{"alpha", "beta"}},
		{name: "positive final line needs no newline", class: hostileCasePositive, request: scannerRequest("tail", hostileInitialBytes, hostileMaximumLineBytes), wantLines: []string{"tail"}},
		{name: "positive UTF-8 width is counted in bytes", class: hostileCasePositive, request: scannerRequest("éclair\n", hostileInitialBytes, hostileMaximumLineBytes), wantLines: []string{"éclair"}},
		{name: "positive embedded NUL remains opaque data", class: hostileCasePositive, request: scannerRequest("a\x00b\n", hostileInitialBytes, hostileMaximumLineBytes), wantLines: []string{"a\x00b"}},
		{name: "positive horizontal whitespace remains data", class: hostileCasePositive, request: scannerRequest("\t value \t\n", hostileInitialBytes, hostileMaximumLineBytes), wantLines: []string{"\t value \t"}},
		{name: "positive empty middle line remains present", class: hostileCasePositive, request: scannerRequest("first\n\nthird\n", hostileInitialBytes, hostileMaximumLineBytes), wantLines: []string{"first", "", "third"}},
		{name: "positive ordinary CRLF is normalized by ScanLines", class: hostileCasePositive, request: scannerRequest("alpha\r\nbeta\r\n", hostileInitialBytes, hostileMaximumLineBytes), wantLines: []string{"alpha", "beta"}},
		{name: "positive one-byte standard reader drives growth", class: hostileCasePositive, request: scannerRequestFrom(func() io.Reader { return iotest.OneByteReader(strings.NewReader("fragmented\n")) }, hostileInitialBytes, hostileMaximumLineBytes), wantLines: []string{"fragmented"}},
		{name: "positive half standard reader preserves all lines", class: hostileCasePositive, request: scannerRequestFrom(func() io.Reader { return iotest.HalfReader(strings.NewReader("one\ntwo\nthree")) }, hostileInitialBytes, hostileMaximumLineBytes), wantLines: []string{"one", "two", "three"}},

		{name: "negative nil reader is rejected at ingress", class: hostileCaseNegative, request: nilScannerRequest(hostileInitialBytes, hostileMaximumLineBytes), wantNewErr: core.ErrLineIOContract},
		{name: "negative typed-nil reader is rejected at ingress", class: hostileCaseNegative, request: typedNilScannerRequest(hostileInitialBytes, hostileMaximumLineBytes), wantNewErr: core.ErrLineIOContract},
		{name: "negative zero buffer policy is rejected", class: hostileCaseNegative, request: scannerRequestWithPolicy("line\n", lineio.BufferPolicy{}), wantNewErr: core.ErrLineIOContract},
		{name: "negative zero initial byte count is rejected", class: hostileCaseNegative, request: scannerRequestWithPolicy("line\n", lineio.BufferPolicy{MaximumLineBytes: mustByteCount(t, hostileMaximumLineBytes)}), wantNewErr: core.ErrLineIOContract},
		{name: "negative zero maximum line byte count is rejected", class: hostileCaseNegative, request: scannerRequestWithPolicy("line\n", lineio.BufferPolicy{InitialBytes: mustByteCount(t, hostileInitialBytes)}), wantNewErr: core.ErrLineIOContract},
		{name: "negative initial buffer above line maximum is rejected", class: hostileCaseNegative, request: scannerRequest("line\n", hostileMaximumLineBytes, hostileInitialBytes), wantNewErr: core.ErrLineIOContract},
		{name: "negative maximum above Lineio ceiling is rejected", class: hostileCaseNegative, request: scannerRequestWithCounts("", mustByteCount(t, 1), maximumSupportedPlusOne), wantNewErr: core.ErrLineIOContract},
		{name: "negative initial above Lineio ceiling is rejected", class: hostileCaseNegative, request: scannerRequestWithCounts("", maximumSupportedPlusOne, maximumSupportedPlusOne), wantNewErr: core.ErrLineIOContract},
		{name: "negative native reader failure is preserved", class: hostileCaseNegative, request: scannerRequestFrom(func() io.Reader { return iotest.ErrReader(hostileReaderError{}) }, hostileInitialBytes, hostileMaximumLineBytes), wantScanErr: core.ErrLineIOScan, wantNativeErr: hostileReaderError{}},
		{name: "negative failure after a complete line preserves prefix and cause", class: hostileCaseNegative, request: scannerRequestFrom(func() io.Reader {
			return io.MultiReader(strings.NewReader("kept\n"), iotest.ErrReader(hostileReaderError{}))
		}, hostileInitialBytes, hostileMaximumLineBytes), wantLines: []string{"kept"}, wantScanErr: core.ErrLineIOScan, wantNativeErr: hostileReaderError{}},

		{name: "boundary empty reader emits no invented line", class: hostileCaseBoundary, request: scannerRequest("", hostileInitialBytes, hostileMaximumLineBytes)},
		{name: "boundary one LF emits one empty line", class: hostileCaseBoundary, request: scannerRequest("\n", hostileInitialBytes, hostileMaximumLineBytes), wantLines: []string{""}},
		{name: "boundary lone CR at EOF normalizes to an empty line", class: hostileCaseBoundary, request: scannerRequest("\r", hostileInitialBytes, hostileMaximumLineBytes), wantLines: []string{""}},
		{name: "boundary two LFs emit two empty lines", class: hostileCaseBoundary, request: scannerRequest("\n\n", hostileInitialBytes, hostileMaximumLineBytes), wantLines: []string{"", ""}},
		{name: "boundary one-byte maximum accepts one byte before LF", class: hostileCaseBoundary, request: scannerRequest("x\n", 1, 1), wantLines: []string{"x"}},
		{name: "boundary one-byte maximum accepts one byte at EOF", class: hostileCaseBoundary, request: scannerRequest("x", 1, 1), wantLines: []string{"x"}},
		{name: "boundary one-byte maximum rejects two bytes before LF", class: hostileCaseBoundary, request: scannerRequest("xy\n", 1, 1), wantScanErr: core.ErrLineIOScan, wantNativeErr: bufio.ErrTooLong},
		{name: "boundary one-byte maximum rejects two bytes at EOF", class: hostileCaseBoundary, request: scannerRequest("xy", 1, 1), wantScanErr: core.ErrLineIOScan, wantNativeErr: bufio.ErrTooLong},
		{name: "boundary one below maximum is accepted", class: hostileCaseBoundary, request: scannerRequest(strings.Repeat("x", hostileMaximumLineBytes-1)+"\n", hostileInitialBytes, hostileMaximumLineBytes), wantLines: []string{strings.Repeat("x", hostileMaximumLineBytes-1)}},
		{name: "boundary exact maximum before LF is accepted", class: hostileCaseBoundary, request: scannerRequest(strings.Repeat("x", hostileMaximumLineBytes)+"\n", hostileInitialBytes, hostileMaximumLineBytes), wantLines: []string{strings.Repeat("x", hostileMaximumLineBytes)}},
		{name: "boundary one above maximum before LF is rejected", class: hostileCaseBoundary, request: scannerRequest(strings.Repeat("x", hostileMaximumLineBytes+1)+"\n", hostileInitialBytes, hostileMaximumLineBytes), wantScanErr: core.ErrLineIOScan, wantNativeErr: bufio.ErrTooLong},
		{name: "boundary exact maximum at EOF is accepted", class: hostileCaseBoundary, request: scannerRequest(strings.Repeat("x", hostileMaximumLineBytes), hostileInitialBytes, hostileMaximumLineBytes), wantLines: []string{strings.Repeat("x", hostileMaximumLineBytes)}},
		{name: "boundary one above maximum at EOF is rejected", class: hostileCaseBoundary, request: scannerRequest(strings.Repeat("x", hostileMaximumLineBytes+1), hostileInitialBytes, hostileMaximumLineBytes), wantScanErr: core.ErrLineIOScan, wantNativeErr: bufio.ErrTooLong},
		{name: "boundary exact maximum before CRLF is accepted", class: hostileCaseBoundary, request: scannerRequest(strings.Repeat("x", hostileMaximumLineBytes)+"\r\n", hostileInitialBytes, hostileMaximumLineBytes), wantLines: []string{strings.Repeat("x", hostileMaximumLineBytes)}},
		{name: "boundary one above maximum before CRLF is rejected", class: hostileCaseBoundary, request: scannerRequest(strings.Repeat("x", hostileMaximumLineBytes+1)+"\r\n", hostileInitialBytes, hostileMaximumLineBytes), wantScanErr: core.ErrLineIOScan, wantNativeErr: bufio.ErrTooLong},
		{name: "boundary initial equal to maximum accepts exact line", class: hostileCaseBoundary, request: scannerRequest(strings.Repeat("x", 8)+"\n", 8, 8), wantLines: []string{strings.Repeat("x", 8)}},
		{name: "boundary line one above initial grows within maximum", class: hostileCaseBoundary, request: scannerRequest(strings.Repeat("x", hostileInitialBytes+1)+"\n", hostileInitialBytes, hostileMaximumLineBytes), wantLines: []string{strings.Repeat("x", hostileInitialBytes+1)}},
		{name: "boundary one below supported policy maximum is admitted", class: hostileCaseBoundary, request: scannerRequestWithCounts("", mustByteCount(t, 1), maximumSupportedMinusOne)},
		{name: "boundary exact supported policy maximum is admitted", class: hostileCaseBoundary, request: scannerRequestWithCounts("", maximumSupported, maximumSupported)},
		{name: "boundary one above supported policy maximum is rejected", class: hostileCaseBoundary, request: scannerRequestWithCounts("", mustByteCount(t, 1), maximumSupportedPlusOne), wantNewErr: core.ErrLineIOContract},
	}

	counts := [hostileCaseLimit]int{}
	for _, tc := range cases {
		if err := tc.class.Validate(); err != nil {
			t.Fatalf("hostile case %q class Validate() error = %v, want nil", tc.name, err)
		}
		counts[tc.class]++
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			scanner, gotNewErr := lineio.New(tc.request(t))
			if tc.wantNewErr != nil {
				if !errors.Is(gotNewErr, tc.wantNewErr) || scanner != nil {
					t.Fatalf("lineio.New() = (%v, %v), want (nil, errors.Is(..., %v))", scanner, gotNewErr, tc.wantNewErr)
				}
				return
			}
			if gotNewErr != nil || scanner == nil {
				t.Fatalf("lineio.New() = (%v, %v), want (scanner, nil)", scanner, gotNewErr)
			}
			gotLines := scanStrings(scanner)
			if !slices.Equal(gotLines, tc.wantLines) {
				t.Fatalf("Scanner lines = %q, want %q", gotLines, tc.wantLines)
			}
			gotScanErr := scanner.Err()
			if !errors.Is(gotScanErr, tc.wantScanErr) {
				t.Fatalf("Scanner.Err() = %v, want errors.Is(..., %v)", gotScanErr, tc.wantScanErr)
			}
			if tc.wantNativeErr != nil && !errors.Is(gotScanErr, tc.wantNativeErr) {
				t.Fatalf("Scanner.Err() = %v, want native errors.Is(..., %v)", gotScanErr, tc.wantNativeErr)
			}
		})
	}

	wantCounts := [hostileCaseLimit]int{
		hostileCasePositive: 10,
		hostileCaseNegative: 10,
		hostileCaseBoundary: 20,
	}
	if counts != wantCounts {
		t.Fatalf("hostile case counts = %v, want %v", counts, wantCounts)
	}
}

func TestScannerStateExhaustive(t *testing.T) {
	t.Parallel()

	request := scannerRequest("line\n", hostileInitialBytes, hostileMaximumLineBytes)(t)
	live, err := lineio.New(request)
	if err != nil {
		t.Fatalf("lineio.New(live) error = %v, want nil", err)
	}
	cases := []struct {
		scanner *lineio.Scanner
		wantErr error
		name    string
	}{
		{name: "nil scanner is invalid", scanner: nil, wantErr: core.ErrLineIOContract},
		{name: "zero scanner is invalid", scanner: &lineio.Scanner{}, wantErr: core.ErrLineIOContract},
		{name: "constructed scanner is valid", scanner: live},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotErr := tc.scanner.Validate()
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("Scanner.Validate() error = %v, want %v", gotErr, tc.wantErr)
			}
			if tc.wantErr != nil {
				if tc.scanner.Scan() || tc.scanner.Bytes() != nil || !errors.Is(tc.scanner.Err(), tc.wantErr) {
					t.Fatalf("invalid Scanner methods = (Scan %t, Bytes %v, Err %v), want (false, nil, %v)", tc.scanner.Scan(), tc.scanner.Bytes(), tc.scanner.Err(), tc.wantErr)
				}
			}
		})
	}
}

func FuzzScannerSemanticClosure(f *testing.F) {
	for _, seed := range [][]byte{
		nil,
		{},
		[]byte("\n"),
		[]byte("x\n"),
		[]byte(strings.Repeat("x", hostileMaximumLineBytes-1) + "\n"),
		[]byte(strings.Repeat("x", hostileMaximumLineBytes) + "\n"),
		[]byte(strings.Repeat("x", hostileMaximumLineBytes+1) + "\n"),
		[]byte(strings.Repeat("x", hostileMaximumLineBytes) + "\r\n"),
		[]byte("first\n\nthird"),
		{0, '\n'},
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		const fuzzInputMaximumBytes = 4 * hostileMaximumLineBytes
		if len(data) > fuzzInputMaximumBytes {
			return
		}
		scanner, err := lineio.New(scannerRequestFrom(func() io.Reader {
			return bytes.NewReader(data)
		}, hostileInitialBytes, hostileMaximumLineBytes)(t))
		if err != nil {
			t.Fatalf("lineio.New(fuzz input) error = %v, want nil", err)
		}
		gotLines := scanStrings(scanner)
		gotErr := scanner.Err()
		wantLines, wantTooLong := independentScanLines(data, hostileMaximumLineBytes)
		if !slices.Equal(gotLines, wantLines) {
			t.Fatalf("Scanner lines = %q, want independent %q", gotLines, wantLines)
		}
		if wantTooLong {
			if !errors.Is(gotErr, core.ErrLineIOScan) || !errors.Is(gotErr, bufio.ErrTooLong) {
				t.Fatalf("Scanner.Err(overlong) = %v, want %v and %v", gotErr, core.ErrLineIOScan, bufio.ErrTooLong)
			}
			return
		}
		if gotErr != nil {
			t.Fatalf("Scanner.Err(accepted) = %v, want nil", gotErr)
		}
	})
}

func scannerRequest(input string, initial, maximum uint64) func(*testing.T) lineio.Request {
	return scannerRequestFrom(func() io.Reader { return strings.NewReader(input) }, initial, maximum)
}

func scannerRequestFrom(source func() io.Reader, initial, maximum uint64) func(*testing.T) lineio.Request {
	return func(t *testing.T) lineio.Request {
		t.Helper()
		return lineio.Request{
			Source: source(),
			Buffer: lineio.BufferPolicy{
				InitialBytes:     mustByteCount(t, initial),
				MaximumLineBytes: mustByteCount(t, maximum),
			},
		}
	}
}

func scannerRequestWithPolicy(input string, policy lineio.BufferPolicy) func(*testing.T) lineio.Request {
	return func(*testing.T) lineio.Request {
		return lineio.Request{Source: strings.NewReader(input), Buffer: policy}
	}
}

func scannerRequestWithCounts(input string, initial, maximum core.ByteCount) func(*testing.T) lineio.Request {
	return func(*testing.T) lineio.Request {
		return lineio.Request{
			Source: strings.NewReader(input),
			Buffer: lineio.BufferPolicy{InitialBytes: initial, MaximumLineBytes: maximum},
		}
	}
}

func nilScannerRequest(initial, maximum uint64) func(*testing.T) lineio.Request {
	return func(t *testing.T) lineio.Request {
		t.Helper()
		return lineio.Request{
			Buffer: lineio.BufferPolicy{InitialBytes: mustByteCount(t, initial), MaximumLineBytes: mustByteCount(t, maximum)},
		}
	}
}

func typedNilScannerRequest(initial, maximum uint64) func(*testing.T) lineio.Request {
	return func(t *testing.T) lineio.Request {
		t.Helper()
		var source *bytes.Buffer
		return lineio.Request{
			Source: source,
			Buffer: lineio.BufferPolicy{InitialBytes: mustByteCount(t, initial), MaximumLineBytes: mustByteCount(t, maximum)},
		}
	}
}

func mustByteCount(t *testing.T, value uint64) core.ByteCount {
	t.Helper()
	count, err := core.NewByteCount(value)
	if err != nil {
		t.Fatalf("core.NewByteCount(%d) error = %v, want nil", value, err)
	}
	return count
}

func scanStrings(scanner *lineio.Scanner) []string {
	var lines []string
	for scanner.Scan() {
		lines = append(lines, string(scanner.Bytes()))
	}
	return lines
}

func independentScanLines(data []byte, maximum int) ([]string, bool) {
	var lines []string
	remaining := data
	for len(remaining) > 0 {
		newline := bytes.IndexByte(remaining, '\n')
		if newline < 0 {
			line := dropFinalCarriageReturn(remaining)
			if len(line) > maximum {
				return lines, true
			}
			return append(lines, string(line)), false
		}
		line := dropFinalCarriageReturn(remaining[:newline])
		if len(line) > maximum {
			return lines, true
		}
		lines = append(lines, string(line))
		remaining = remaining[newline+1:]
	}
	return lines, false
}

func dropFinalCarriageReturn(line []byte) []byte {
	if len(line) > 0 && line[len(line)-1] == '\r' {
		return line[:len(line)-1]
	}
	return line
}
