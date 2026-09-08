package filestore

import (
	"bytes"
	"errors"
	"io"
	"os"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// Each operation creates and synchronizes exactly 128 bytes, opens the staged
// inode, reads and compares all bytes, closes it, then durably discards its name.
// File effects are the declared workload; fixture storage is reused.
func BenchmarkStageReadDiscardExactBytes(b *testing.B) {
	directory := b.TempDir()
	root, err := os.OpenRoot(directory)
	if err != nil {
		b.Fatal(err)
	}
	defer func() {
		if err := root.Close(); err != nil {
			b.Error(err)
		}
	}()
	path, err := core.ParseRelativePath("stage")
	if err != nil {
		b.Fatal(err)
	}
	payload := bytes.Repeat([]byte{0, 255, 7, 31}, 32)
	maximum, err := core.NewByteCount(uint64(len(payload)))
	if err != nil {
		b.Fatal(err)
	}
	var source bytes.Reader
	gotBytes := make([]byte, len(payload)+1)
	request := StageRequest{Source: &source, Temporary: Location{Root: root, Path: path}, Mode: 0o600, MaximumBytes: maximum}
	if err := request.Validate(); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.SetBytes(int64(len(payload)))
	for b.Loop() {
		source.Reset(payload)
		staged, err := Stage(b.Context(), request)
		if err != nil || staged.BytesWritten().Uint64() != uint64(len(payload)) {
			b.Fatalf("Stage = (%+v,%v), want exact extent", staged, err)
		}
		file, err := OpenStagedRead(b.Context(), staged)
		if err != nil {
			b.Fatal(err)
		}
		n, readErr := io.ReadFull(file, gotBytes)
		closeErr := file.Close()
		if !errors.Is(readErr, io.ErrUnexpectedEOF) || closeErr != nil || n != len(payload) || !bytes.Equal(gotBytes[:n], payload) {
			b.Fatalf("read = (%d,%v,%v,%v), want exact bytes and EOF", n, gotBytes[:n], readErr, closeErr)
		}
		if err := Discard(b.Context(), staged); err != nil {
			b.Fatal(err)
		}
	}
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) != 0 {
		b.Fatalf("remaining entries = (%v,%v), want empty namespace", entries, err)
	}
}

// This measures public acquisition, bounded transfer, exact receipt and byte
// comparison, and close. The same 128-byte file is read on every operation.
func BenchmarkReadExistingExactBytes(b *testing.B) {
	directory := b.TempDir()
	root, err := os.OpenRoot(directory)
	if err != nil {
		b.Fatal(err)
	}
	defer func() {
		if err := root.Close(); err != nil {
			b.Error(err)
		}
	}()
	path, err := core.ParseRelativePath("source")
	if err != nil {
		b.Fatal(err)
	}
	payload := bytes.Repeat([]byte{0, 255, 7, 31}, 32)
	if err := os.WriteFile(directory+"/"+path.String(), payload, 0o600); err != nil {
		b.Fatal(err)
	}
	maximum, err := core.NewByteCount(uint64(len(payload)))
	if err != nil {
		b.Fatal(err)
	}
	var destination bytes.Buffer
	destination.Grow(len(payload))
	request := ReadRequest{Destination: &destination, Location: Location{Root: root, Path: path}, MaximumBytes: maximum}
	if err := request.Validate(); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.SetBytes(int64(len(payload)))
	for b.Loop() {
		destination.Reset()
		got, err := Read(b.Context(), request)
		if err != nil || got.Uint64() != uint64(len(payload)) || !bytes.Equal(destination.Bytes(), payload) {
			b.Fatalf("Read = (%d,%v,%v), want exact %v", got.Uint64(), destination.Bytes(), err, payload)
		}
	}
	retained, err := os.ReadFile(directory + "/" + path.String())
	if err != nil || !bytes.Equal(retained, payload) {
		b.Fatalf("retained source = (%v,%v), want %v", retained, err, payload)
	}
}
