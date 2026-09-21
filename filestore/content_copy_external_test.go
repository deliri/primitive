package filestore_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

func contentCopyFixture(t testing.TB, data []byte) filestore.ContentIndexEntry {
	t.Helper()
	extent, err := core.NewByteLength(uint64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	return filestore.ContentIndexEntry{Digest: core.NewSHA256Digest(sha256.Sum256(data)), Extent: extent}
}

func TestCopyContentStreamLayerTriadPreservesDeclaredContentAcrossWindows(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		size      int
		agreement bool
	}{
		{name: "empty observation without declaration", size: 0, agreement: false},
		{name: "empty exact agreement", size: 0, agreement: true},
		{name: "one byte before first full window", size: 1, agreement: true},
		{name: "one below working window", size: 7, agreement: true},
		{name: "exact working window", size: 8, agreement: true},
		{name: "one beyond working window", size: 9, agreement: true},
		{name: "several complete windows", size: 24, agreement: true},
		{name: "final partial window", size: 25, agreement: true},
		{name: "undeclared extent continues beyond window", size: 1025, agreement: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			data := bytes.Repeat([]byte{0x8e}, tc.size)
			want := contentCopyFixture(t, data)
			var destination bytes.Buffer
			var buffer [8]byte
			request := filestore.CopyContentRequest{Source: bytes.NewReader(data), Destination: &destination, Buffer: buffer[:]}
			if tc.agreement {
				request.Expected = &want
			}
			got, err := filestore.CopyContent(t.Context(), request)
			if err != nil || got != want || !bytes.Equal(destination.Bytes(), data) {
				t.Fatalf("copy = (%v,%v) bytes=%x, want %v and exact source", got, err, destination.Bytes(), want)
			}
			if err := got.Validate(); err != nil {
				t.Fatalf("observed content validation = %v, want nil", err)
			}
		})
	}
}

func TestCopyContentSourceLayerTriadPreservesReaderFaultsAndRefusesPartialProof(t *testing.T) {
	t.Parallel()
	data := []byte("abc")
	cases := []struct {
		name      string
		source    func() io.Reader
		wantErr   error
		wantBytes string
	}{
		{name: "ordinary EOF commits exact bytes", source: func() io.Reader { return bytes.NewReader(data) }, wantErr: nil, wantBytes: "abc"},
		{name: "data and EOF in one read", source: func() io.Reader { return iotest.DataErrReader(bytes.NewReader(data)) }, wantErr: nil, wantBytes: "abc"},
		{name: "short successful reads conserve bytes", source: func() io.Reader { return iotest.HalfReader(bytes.NewReader(data)) }, wantErr: nil, wantBytes: "abc"},
		{name: "short declared source remains a source failure", source: func() io.Reader { return strings.NewReader("ab") }, wantErr: core.ErrFilestoreSource, wantBytes: "ab"},
		{name: "source exceeds exact declaration", source: func() io.Reader { return strings.NewReader("abcd") }, wantErr: core.ErrFilestoreSize, wantBytes: "abc"},
		{name: "same extent changed content refuses digest proof", source: func() io.Reader { return strings.NewReader("abd") }, wantErr: core.ErrFilestoreContract, wantBytes: "abd"},
		{name: "native source refusal before progress", source: func() io.Reader { return iotest.ErrReader(io.ErrClosedPipe) }, wantErr: io.ErrClosedPipe, wantBytes: ""},
		{name: "source fails after declared bytes", source: func() io.Reader { return io.MultiReader(bytes.NewReader(data), iotest.ErrReader(io.ErrClosedPipe)) }, wantErr: io.ErrClosedPipe, wantBytes: "abc"},
		{name: "wrapped EOF is a failure", source: func() io.Reader {
			return io.MultiReader(bytes.NewReader(data), iotest.ErrReader(errors.Join(io.EOF, io.ErrClosedPipe)))
		}, wantErr: io.ErrClosedPipe, wantBytes: "abc"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			want := contentCopyFixture(t, data)
			var destination bytes.Buffer
			got, err := filestore.CopyContent(t.Context(), filestore.CopyContentRequest{Source: tc.source(), Destination: &destination, Expected: &want})
			if !errors.Is(err, tc.wantErr) || destination.String() != tc.wantBytes {
				t.Fatalf("copy = (%v,%v) bytes=%q, want error %v and bytes %q", got, err, destination.String(), tc.wantErr, tc.wantBytes)
			}
			if tc.wantErr == nil && got != want || tc.wantErr != nil && got != (filestore.ContentIndexEntry{}) {
				t.Fatalf("content proof = %v with error %v, want exact success or zero refusal", got, err)
			}
		})
	}
}

type copyContentFaultWriter struct {
	cause error
	count int
}

func (w copyContentFaultWriter) Write([]byte) (int, error) { return w.count, w.cause }

func TestCopyContentDestinationLayerTriadPreservesNativeFailureAndShortWrite(t *testing.T) {
	t.Parallel()
	cases := []struct {
		writer        io.Writer
		wantErr       error
		name          string
		wantRemaining int
	}{
		{name: "nil destination refuses before source", writer: nil, wantErr: core.ErrFilestoreContract, wantRemaining: 3},
		{name: "typed nil destination refuses before source", writer: (*bytes.Buffer)(nil), wantErr: core.ErrFilestoreContract, wantRemaining: 3},
		{name: "native destination error with partial progress", writer: copyContentFaultWriter{count: 1, cause: io.ErrClosedPipe}, wantErr: io.ErrClosedPipe, wantRemaining: 0},
		{name: "silent short write is not success", writer: copyContentFaultWriter{count: 1}, wantErr: io.ErrShortWrite, wantRemaining: 0},
		{name: "negative write count is refused", writer: copyContentFaultWriter{count: -1}, wantErr: core.ErrFilestoreDestination, wantRemaining: 0},
		{name: "write count exceeds supplied buffer", writer: copyContentFaultWriter{count: 100}, wantErr: core.ErrFilestoreDestination, wantRemaining: 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			source := strings.NewReader("abc")
			got, err := filestore.CopyContent(t.Context(), filestore.CopyContentRequest{Source: source, Destination: tc.writer})
			if !errors.Is(err, tc.wantErr) || got != (filestore.ContentIndexEntry{}) {
				t.Fatalf("destination refusal = (%v,%v), want zero and %v", got, err, tc.wantErr)
			}
			if source.Len() != tc.wantRemaining {
				t.Fatalf("source remaining=%d, want %d", source.Len(), tc.wantRemaining)
			}
		})
	}
}

func TestCopyContentAdmissionLayerTriadNoReadOrWriteOnRefusal(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	cases := []struct {
		ctx       context.Context
		source    io.Reader
		wantErr   error
		agreement *filestore.ContentIndexEntry
		name      string
	}{
		{name: "cancelled before first read", ctx: ctx, source: strings.NewReader("abc"), wantErr: context.Canceled},
		{name: "nil context is refused", source: strings.NewReader("abc"), wantErr: core.ErrNilContext},
		{name: "nil source is refused", ctx: t.Context(), wantErr: core.ErrFilestoreContract},
		{name: "typed nil source is refused", ctx: t.Context(), source: (*bytes.Reader)(nil), wantErr: core.ErrFilestoreContract},
		{name: "unset expected digest is refused", ctx: t.Context(), source: strings.NewReader("abc"), agreement: &filestore.ContentIndexEntry{}, wantErr: core.ErrFilestoreContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var destination bytes.Buffer
			got, err := filestore.CopyContent(tc.ctx, filestore.CopyContentRequest{Source: tc.source, Destination: &destination, Expected: tc.agreement})
			if !errors.Is(err, tc.wantErr) || got != (filestore.ContentIndexEntry{}) || destination.Len() != 0 {
				t.Fatalf("admission = (%v,%v) output=%x, want zero/no writes and %v", got, err, destination.Bytes(), tc.wantErr)
			}
		})
	}
}

func FuzzCopyContentExactAgreement(f *testing.F) {
	f.Add([]byte("abc"), uint8(0))
	f.Add([]byte{}, uint8(0))
	f.Add([]byte("abc"), uint8(1))
	f.Add([]byte("abc"), uint8(2))
	f.Fuzz(func(t *testing.T, data []byte, mutation uint8) {
		want := contentCopyFixture(t, data)
		expected := want
		switch mutation % 3 {
		case 0:
		case 1:
			raw := sha256.Sum256(data)
			raw[0] ^= 1
			expected.Digest = core.NewSHA256Digest(raw)
		case 2:
			length, err := core.NewByteLength(uint64(len(data)) + 1)
			if err != nil {
				t.Fatal(err)
			}
			expected.Extent = length
		}
		if mutation%3 != 0 && expected == want {
			t.Fatalf("mutated agreement=%v, want a change from %v", expected, want)
		}
		destination := sha256.New()
		got, err := filestore.CopyContent(t.Context(), filestore.CopyContentRequest{Source: bytes.NewReader(data), Destination: destination, Expected: &expected})
		if mutation%3 == 0 {
			if err != nil || got != want {
				t.Fatalf("exact copy = (%v,%v), want %v", got, err, want)
			}
			raw := sha256.Sum256(data)
			if !bytes.Equal(destination.Sum(nil), raw[:]) {
				t.Fatalf("output digest=%x, want %x", destination.Sum(nil), raw)
			}
			return
		}
		wantErr := core.ErrFilestoreContract
		if mutation%3 == 2 {
			wantErr = core.ErrFilestoreSource
		}
		if !errors.Is(err, wantErr) || got != (filestore.ContentIndexEntry{}) {
			t.Fatalf("mutated agreement = (%v,%v), want zero and %v", got, err, wantErr)
		}
	})
}
