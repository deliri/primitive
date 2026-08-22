package core

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

const strictJSONReaderTestDocumentMaximumBytes = 64

type strictJSONReaderTestError struct{}

func (strictJSONReaderTestError) Error() string {
	return "strict JSON reader test failure"
}

type strictJSONFragmentedReader struct {
	data     []byte
	fragment int
}

func (r *strictJSONFragmentedReader) Read(destination []byte) (int, error) {
	if len(r.data) == 0 {
		return 0, io.EOF
	}
	count := min(len(r.data), r.fragment, len(destination))
	copy(destination, r.data[:count])
	r.data = r.data[count:]
	return count, nil
}

type strictJSONTerminalReader struct {
	data []byte
	done bool
}

func (r *strictJSONTerminalReader) Read(destination []byte) (int, error) {
	if r.done {
		return 0, io.EOF
	}
	r.done = true
	count := copy(destination, r.data)
	return count, io.EOF
}

type strictJSONFailureReader struct {
	data []byte
	done bool
}

func (r *strictJSONFailureReader) Read(destination []byte) (int, error) {
	if r.done {
		return 0, strictJSONReaderTestError{}
	}
	r.done = true
	return copy(destination, r.data), nil
}

type strictJSONJoinedTerminalFailureReader struct {
	data []byte
	done bool
}

func (r *strictJSONJoinedTerminalFailureReader) Read(destination []byte) (int, error) {
	if r.done {
		return 0, io.EOF
	}
	r.done = true
	return copy(destination, r.data), errors.Join(io.EOF, strictJSONReaderTestError{})
}

type strictJSONEmptyReader struct{}

func (strictJSONEmptyReader) Read([]byte) (int, error) {
	return 0, nil
}

type strictJSONDelayedReader struct {
	source     io.Reader
	emptyReads int
}

func (r *strictJSONDelayedReader) Read(destination []byte) (int, error) {
	if r.emptyReads > 0 {
		r.emptyReads--
		return 0, nil
	}
	return r.source.Read(destination)
}

type strictJSONImpossibleCountReader struct {
	count int
}

func (r strictJSONImpossibleCountReader) Read(destination []byte) (int, error) {
	if r.count < 0 {
		return r.count, nil
	}
	return len(destination) + r.count, nil
}

type strictJSONReadForbidden struct{}

func (strictJSONReadForbidden) Read([]byte) (int, error) {
	return 0, strictJSONReaderTestError{}
}

func TestDecodeStrictJSONReaderHostileBoundaryTable(t *testing.T) {
	t.Parallel()

	canonical := strictJSONReaderCanonicalAbsolutePath(t)
	limits := strictJSONReaderLimits(t)
	cases := []struct {
		reader  func() io.Reader
		limits  StrictJSONLimits
		name    string
		wantErr error
		forbid  error
	}{
		{
			name: "one byte below document limit is accepted",
			reader: func() io.Reader {
				return bytes.NewReader(strictJSONReaderPadded(canonical, strictJSONReaderTestDocumentMaximumBytes-1))
			},
			limits: limits,
		},
		{
			name: "exact document limit is accepted",
			reader: func() io.Reader {
				return bytes.NewReader(strictJSONReaderPadded(canonical, strictJSONReaderTestDocumentMaximumBytes))
			},
			limits: limits,
		},
		{
			name: "one byte above document limit is rejected",
			reader: func() io.Reader {
				return bytes.NewReader(strictJSONReaderPadded(canonical, strictJSONReaderTestDocumentMaximumBytes+1))
			},
			limits:  limits,
			wantErr: ErrJSONContract,
		},
		{
			name:   "single byte fragments are accepted",
			reader: func() io.Reader { return &strictJSONFragmentedReader{data: bytes.Clone(canonical), fragment: 1} },
			limits: limits,
		},
		{
			name:   "data and terminal EOF in one read are accepted",
			reader: func() io.Reader { return &strictJSONTerminalReader{data: bytes.Clone(canonical)} },
			limits: limits,
		},
		{
			name:    "native failure after bytes remains reachable",
			reader:  func() io.Reader { return &strictJSONFailureReader{data: bytes.Clone(canonical[:1])} },
			limits:  limits,
			wantErr: strictJSONReaderTestError{},
		},
		{
			name: "native failure joined with EOF remains reachable",
			reader: func() io.Reader {
				return &strictJSONJoinedTerminalFailureReader{data: bytes.Clone(canonical)}
			},
			limits:  limits,
			wantErr: strictJSONReaderTestError{},
		},
		{
			name: "one below empty read limit is accepted",
			reader: func() io.Reader {
				return &strictJSONDelayedReader{
					source: bytes.NewReader(canonical), emptyReads: ReaderConsecutiveEmptyReadMaximum - 1,
				}
			},
			limits: limits,
		},
		{
			name:    "repeated empty reads are bounded",
			reader:  func() io.Reader { return strictJSONEmptyReader{} },
			limits:  limits,
			wantErr: io.ErrNoProgress,
		},
		{
			name:    "reader reporting a negative count is rejected",
			reader:  func() io.Reader { return strictJSONImpossibleCountReader{count: -1} },
			limits:  limits,
			wantErr: ErrJSONContract,
		},
		{
			name:    "reader reporting beyond its buffer is rejected",
			reader:  func() io.Reader { return strictJSONImpossibleCountReader{count: 1} },
			limits:  limits,
			wantErr: ErrJSONContract,
		},
		{
			name:    "invalid limits are rejected before reading",
			reader:  func() io.Reader { return strictJSONReadForbidden{} },
			limits:  StrictJSONLimits{},
			wantErr: ErrJSONContract,
			forbid:  strictJSONReaderTestError{},
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			got, gotErr := DecodeStrictJSON[AbsolutePath](testCase.reader(), testCase.limits)
			if testCase.wantErr == nil {
				if gotErr != nil || got.Validate() != nil {
					t.Fatalf("DecodeStrictJSON() = (%v, %v), want valid path and nil", got, gotErr)
				}
				return
			}
			if !errors.Is(gotErr, ErrJSONContract) ||
				!errors.Is(gotErr, testCase.wantErr) ||
				errors.Is(gotErr, testCase.forbid) ||
				got != (AbsolutePath{}) {
				t.Fatalf("DecodeStrictJSON() = (%v, %v), want zero and error including %v", got, gotErr, testCase.wantErr)
			}
		})
	}
}

func TestDecodeStrictJSONReaderRejectsNilReaders(t *testing.T) {
	t.Parallel()

	var typedNil *bytes.Reader
	cases := []struct {
		reader io.Reader
		name   string
	}{
		{name: "nil interface", reader: nil},
		{name: "typed nil pointer", reader: typedNil},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			got, gotErr := DecodeStrictJSON[AbsolutePath](testCase.reader, DefaultStrictJSONLimits())
			if !errors.Is(gotErr, ErrJSONContract) || got != (AbsolutePath{}) {
				t.Fatalf("DecodeStrictJSON() = (%v, %v), want zero and %v", got, gotErr, ErrJSONContract)
			}
		})
	}
}

func FuzzDecodeStrictJSONReaderAbsolutePathPublicBoundary(f *testing.F) {
	canonical := strictJSONReaderCanonicalAbsolutePath(f)
	f.Add(canonical, uint8(1))
	f.Add([]byte{}, uint8(1))
	f.Add([]byte("null"), uint8(2))
	f.Add([]byte(`{"path":"/tmp/value"}`), uint8(3))

	f.Fuzz(func(t *testing.T, wire []byte, rawFragment uint8) {
		limits := DefaultStrictJSONLimits()
		documentMaximum, gotLimitErr := NewByteCount(fuzzjsonDocumentMaximumBytes)
		if gotLimitErr != nil {
			t.Fatalf("NewByteCount(%d) error = %v, want nil", fuzzjsonDocumentMaximumBytes, gotLimitErr)
		}
		limits.DocumentMaximumBytes = documentMaximum
		fragment := int(rawFragment%16) + 1
		got, gotErr := DecodeStrictJSON[AbsolutePath](
			&strictJSONFragmentedReader{data: wire, fragment: fragment},
			limits,
		)
		if gotErr != nil {
			if !errors.Is(gotErr, ErrJSONContract) || got != (AbsolutePath{}) {
				t.Fatalf("DecodeStrictJSON(rejected) = (%v, %v), want zero and %v", got, gotErr, ErrJSONContract)
			}
			return
		}
		if gotValidateErr := got.Validate(); gotValidateErr != nil {
			t.Fatalf("DecodeStrictJSON(accepted).Validate() error = %v, want nil", gotValidateErr)
		}
		encoded, gotEncodeErr := EncodeValidatedJSON(got, limits)
		if gotEncodeErr != nil {
			t.Fatalf("EncodeValidatedJSON(accepted) error = %v, want nil", gotEncodeErr)
		}
		roundTrip, gotRoundTripErr := DecodeStrictJSON[AbsolutePath](bytes.NewReader(encoded), limits)
		if gotRoundTripErr != nil || roundTrip != got {
			t.Fatalf("DecodeStrictJSON(round trip) = (%v, %v), want (%v, nil)", roundTrip, gotRoundTripErr, got)
		}
	})
}

func strictJSONReaderCanonicalAbsolutePath(tb testing.TB) []byte {
	tb.Helper()
	path, err := ParseAbsolutePath("/tmp/primitive-strict-json-reader")
	if err != nil {
		tb.Fatalf("ParseAbsolutePath() error = %v, want nil", err)
	}
	encoded, err := path.MarshalJSON()
	if err != nil {
		tb.Fatalf("AbsolutePath.MarshalJSON() error = %v, want nil", err)
	}
	return encoded
}

func strictJSONReaderLimits(tb testing.TB) StrictJSONLimits {
	tb.Helper()
	maximum, err := NewByteCount(strictJSONReaderTestDocumentMaximumBytes)
	if err != nil {
		tb.Fatalf("NewByteCount(%d) error = %v, want nil", strictJSONReaderTestDocumentMaximumBytes, err)
	}
	limits := DefaultStrictJSONLimits()
	limits.DocumentMaximumBytes = maximum
	return limits
}

func strictJSONReaderPadded(canonical []byte, size int) []byte {
	return append(bytes.Clone(canonical), strings.Repeat(" ", size-len(canonical))...)
}
