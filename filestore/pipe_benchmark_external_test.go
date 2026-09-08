package filestore_test

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/deliri/primitive/v2026/filestore"
)

// Fixed acquisition -> 128-byte binary transfer -> writer close -> exact EOF
// -> reader close. Both acquisition and native I/O are intentionally timed.
func BenchmarkPipeAcquireBinaryEOFAndClose(b *testing.B) {
	payload := bytes.Repeat([]byte{0, 255, 1, 127}, 32)
	got := make([]byte, len(payload))
	var extra [1]byte
	b.ReportAllocs()
	for b.Loop() {
		pipe, err := filestore.OpenPipe(b.Context())
		if err != nil || pipe.Validate() != nil {
			b.Fatalf("pipe = (%+v,%v), want admitted custody", pipe, err)
		}
		n, writeErr := pipe.Writer.Write(payload)
		closeWriterErr := pipe.Writer.Close()
		read, readErr := io.ReadFull(pipe.Reader, got)
		eofN, eofErr := pipe.Reader.Read(extra[:])
		closeReaderErr := pipe.Reader.Close()
		if n != len(payload) || writeErr != nil || closeWriterErr != nil || read != len(payload) || readErr != nil || !bytes.Equal(got, payload) || eofN != 0 || !errors.Is(eofErr, io.EOF) || closeReaderErr != nil {
			b.Fatalf("pipe lifecycle = (%d,%v,%v,%d,%v,%v,%d,%v,%v), want exact binary stream and EOF", n, writeErr, closeWriterErr, read, readErr, got, eofN, eofErr, closeReaderErr)
		}
	}
}

// Reused endpoints carry exactly 512 bytes per operation. This measures Go's
// transferred native capability; OpenPipe construction and final EOF are untimed.
func BenchmarkPipeHeldBinaryTransfer(b *testing.B) {
	payload := bytes.Repeat([]byte{0, 255, 1, 127}, 128)
	got := make([]byte, len(payload))
	pipe, err := filestore.OpenPipe(b.Context())
	if err != nil || pipe.Validate() != nil {
		b.Fatalf("pipe = (%+v,%v), want admitted custody", pipe, err)
	}
	writerClosed := false
	b.Cleanup(func() {
		if err := pipe.Reader.Close(); err != nil {
			b.Error(err)
		}
		if !writerClosed {
			if err := pipe.Writer.Close(); err != nil {
				b.Error(err)
			}
		}
	})
	finish := newPipeBackstop(b, pipe)
	defer func() {
		if err := finish(); err != nil {
			b.Error(err)
		}
	}()
	b.ReportAllocs()
	b.SetBytes(int64(len(payload)))
	for b.Loop() {
		if n, err := pipe.Writer.Write(payload); err != nil || n != len(payload) {
			b.Fatalf("native write = (%d,%v), want %d", n, err, len(payload))
		}
		if n, err := io.ReadFull(pipe.Reader, got); err != nil || n != len(payload) || !bytes.Equal(got, payload) {
			b.Fatalf("native read = (%v,%d,%v), want %v", got, n, err, payload)
		}
	}
	if err := pipe.Writer.Close(); err != nil {
		b.Fatal(err)
	}
	writerClosed = true
	var extra [1]byte
	if n, err := pipe.Reader.Read(extra[:]); n != 0 || !errors.Is(err, io.EOF) {
		b.Fatalf("completed pipe = (%d,%v), want exact EOF", n, err)
	}
}
