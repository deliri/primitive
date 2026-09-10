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

// This target varies the actual byte stream, read fragmentation,
// unrelated Len declaration and terminal native cause. Its oracle derives the
// exact transferred bytes from the input, not from production's receipt.
func FuzzStreamCopySourceSemanticCustody(f *testing.F) {
	emitted, err := FilestoreRoundTripSeedForTest(f.Context(), f.TempDir())
	if err != nil {
		f.Fatal(err)
	}
	f.Add(emitted, uint8(0), int16(0), false, uint16(128))
	for _, seed := range []struct {
		payload  []byte
		fragment uint8
		declared int16
		failure  bool
	}{
		{payload: nil, fragment: 0},
		{payload: []byte{0, 255}, fragment: 0},
		{payload: []byte{0, 255, 7}, fragment: 2},
		{payload: []byte{0, 255}, fragment: 1, declared: -1},
		{payload: []byte{0, 255}, fragment: 2, failure: true},
		{payload: nil, fragment: 1, failure: true},
	} {
		f.Add(seed.payload, seed.fragment, seed.declared, seed.failure, uint16(seed.fragment))
	}
	f.Fuzz(func(t *testing.T, payload []byte, rawFragment uint8, declared int16, failure bool, rawWindow uint16) {
		payload = payload[:min(len(payload), 4096)]
		// A joined EOF must retain this exact native typed error object.
		native := &fs.PathError{Op: "read", Path: "caller-source", Err: fs.ErrPermission}
		var terminal error = io.EOF
		if failure {
			terminal = errors.Join(io.EOF, native)
		}
		source := misleadingLengthReader{Reader: fragmentedFuzzReader{source: io.MultiReader(bytes.NewReader(payload), &terminalCopyReader{err: terminal}), fragment: int(rawFragment) + 1}, remaining: int(declared)}
		var destination bytes.Buffer
		got, gotErr := copyStream(streamCopyRequest{ctx: t.Context(), source: source, destination: &destination, kind: streamDestinationCaller, buffer: make([]byte, rawWindow%1025)})
		wantCount := uint64(len(payload))
		if got.Uint64() != wantCount || !bytes.Equal(destination.Bytes(), payload[:wantCount]) {
			t.Fatalf("receipt/bytes = (%d,%v), want (%d,%v)", got.Uint64(), destination.Bytes(), wantCount, payload[:wantCount])
		}
		switch {
		case failure:
			var gotNative *fs.PathError
			if !errors.Is(gotErr, core.ErrFilestoreSource) || !errors.Is(gotErr, native) || !errors.As(gotErr, &gotNative) || gotNative != native {
				t.Fatalf("terminal refusal = %v, want exact native object %v and source identity", gotErr, native)
			}
		default:
			if gotErr != nil {
				t.Fatalf("valid stream error = %v, want nil", gotErr)
			}
		}
	})
}
