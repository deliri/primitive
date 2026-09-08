package filestore

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

type fragmentedFuzzReader struct {
	source   io.Reader
	fragment int
}

func (r fragmentedFuzzReader) Read(p []byte) (int, error) {
	return r.source.Read(p[:min(len(p), r.fragment)])
}

// This target varies the actual byte stream, read fragmentation, size ceiling,
// unrelated Len declaration and terminal native cause. Its oracle derives the
// exact retained prefix from the input, not from production's receipt.
func FuzzBoundedCopySourceSemanticCustody(f *testing.F) {
	emitted, err := FilestoreRoundTripSeedForTest(f.Context(), f.TempDir())
	if err != nil {
		f.Fatal(err)
	}
	f.Add(emitted, uint16(len(emitted)-1), uint8(0), int16(0), false)
	for _, seed := range []struct {
		payload  []byte
		maximum  uint16
		fragment uint8
		declared int16
		failure  bool
	}{
		{payload: nil, maximum: 0, fragment: 0},
		{payload: []byte{0, 255}, maximum: 1, fragment: 0},
		{payload: []byte{0, 255, 7}, maximum: 1, fragment: 2},
		{payload: []byte{0, 255}, maximum: 2, fragment: 1, declared: -1},
		{payload: []byte{0, 255}, maximum: 1, fragment: 2, failure: true},
		{payload: nil, maximum: 1, fragment: 1, failure: true},
	} {
		f.Add(seed.payload, seed.maximum, seed.fragment, seed.declared, seed.failure)
	}
	f.Fuzz(func(t *testing.T, payload []byte, rawMaximum uint16, rawFragment uint8, declared int16, failure bool) {
		payload = payload[:min(len(payload), 4096)]
		ceiling := uint64(rawMaximum%4097) + 1
		maximum, err := core.NewByteCount(ceiling)
		if err != nil {
			t.Fatal(err)
		}
		// A joined EOF must retain this exact native typed error object.
		native := &fs.PathError{Op: "read", Path: "caller-source", Err: fs.ErrPermission}
		var terminal error = io.EOF
		if failure {
			terminal = errors.Join(io.EOF, native)
		}
		source := misleadingLengthReader{Reader: fragmentedFuzzReader{source: io.MultiReader(bytes.NewReader(payload), &terminalCopyReader{err: terminal}), fragment: int(rawFragment) + 1}, remaining: int(declared)}
		var destination bytes.Buffer
		got, gotErr := copyBounded(boundedCopyRequest{ctx: t.Context(), source: source, destination: &destination, maximum: maximum, kind: streamDestinationCaller})
		wantCount := min(uint64(len(payload)), ceiling)
		if got.Uint64() != wantCount || !bytes.Equal(destination.Bytes(), payload[:wantCount]) {
			t.Fatalf("receipt/bytes = (%d,%v), want (%d,%v)", got.Uint64(), destination.Bytes(), wantCount, payload[:wantCount])
		}
		switch {
		case uint64(len(payload)) > ceiling:
			if !errors.Is(gotErr, core.ErrFilestoreSize) || errors.Is(gotErr, native) {
				t.Fatalf("overflow = %v, want size refusal without unobserved terminal cause", gotErr)
			}
		case failure:
			var gotNative *fs.PathError
			if !errors.Is(gotErr, core.ErrFilestoreSource) || !errors.Is(gotErr, native) || !errors.As(gotErr, &gotNative) || gotNative != native {
				t.Fatalf("terminal refusal = %v, want exact native object %v and source identity", gotErr, native)
			}
		default:
			if gotErr != nil {
				t.Fatalf("bounded valid stream error = %v, want nil", gotErr)
			}
		}
	})
}
