package exchange

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// Measures the complete buffer/callback/release operation. Source bytes and the
// sealed socket are fixed setup; each operation owns a fresh private response
// buffer. The destination is reset inside timing and checks exact acknowledged
// bytes on every iteration. This measures bounded aggregation, not streaming.
func BenchmarkResponseBufferRelease(b *testing.B) {
	b.ReportAllocs()
	cases := []struct {
		name string
		size int
	}{
		{name: "128B", size: 128},
		{name: "4KiB", size: 4096},
		{name: "64KiB", size: 65536},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			payload := bytes.Repeat([]byte{0x5a}, tc.size)
			maximum, err := core.NewByteCount(uint64(tc.size))
			if err != nil {
				b.Fatal(err)
			}
			destination := &materialFuzzWriter{header: make(http.Header)}
			call, err := NewSocketServerCall(destination, httptest.NewRequestWithContext(b.Context(), http.MethodGet, "/", nil))
			if err != nil {
				b.Fatal(err)
			}
			callbacks := 0
			request := ResponseBufferRequest{Call: call, BodyMaximum: maximum, Serve: func(socket SocketServerCall) error {
				callbacks++
				socket.writer.WriteHeader(http.StatusOK)
				written, err := socket.writer.Write(payload)
				if written != len(payload) && err == nil {
					return io.ErrShortWrite
				}
				return err
			}}
			if err := request.Validate(); err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.SetBytes(int64(tc.size))
			for b.Loop() {
				destination.body.Reset()
				destination.commits, destination.writes = 0, 0
				before := callbacks
				result, err := BufferResponse(b.Context(), request)
				if err != nil || !result.Committed || result.Status != core.HTTPStatusOK() || result.Bytes.Uint64() != uint64(tc.size) || callbacks != before+1 || destination.commits != 1 || destination.writes != 1 || !bytes.Equal(destination.body.Bytes(), payload) {
					b.Fatalf("buffer receipt=(%+v,%v),want exact callback/release and %d bytes", result, err, tc.size)
				}
			}
		})
	}
}
