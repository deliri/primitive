package filestore

import (
	"bufio"
	"bytes"
	"errors"
	"math"
	"os"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestAppendAcceptanceRetainsPartialGoCountBesideNativeFailure(t *testing.T) {
	t.Parallel()
	owned, err := os.Create(t.TempDir() + "/owned")
	if err != nil {
		t.Fatal(err)
	}
	oracleFile, err := os.Create(t.TempDir() + "/oracle")
	if err != nil {
		t.Fatal(err)
	}
	d := &AppendDestination{file: owned, writer: bufio.NewWriter(owned)}
	d.self = d
	oracle := bufio.NewWriter(oracleFile)
	prefix := []byte("prefix!")
	n, err := d.Write(prefix)
	if err != nil {
		t.Fatal(err)
	}
	want, err := oracle.Write(prefix)
	if err != nil || n != want {
		t.Fatalf("prefix=%d/%v,want%d", n, err, want)
	}
	if err := owned.Close(); err != nil {
		t.Fatal(err)
	}
	if err := oracleFile.Close(); err != nil {
		t.Fatal(err)
	}
	payload := bytes.Repeat([]byte{'x'}, 8192)
	n, err = d.Write(payload)
	want, wantErr := oracle.Write(payload)
	if n != want || n <= 0 || n >= len(payload) || !errors.Is(err, os.ErrClosed) || !errors.Is(wantErr, os.ErrClosed) {
		t.Fatalf("partial native=%d/%v,Go oracle=%d/%v,want positive partial acceptance beside closed cause", n, err, want, wantErr)
	}
	got, observeErr := d.AcceptedBytes()
	if observeErr != nil || got.Uint64() != uint64(len(prefix)+want) {
		t.Fatalf("partial acceptance=%v/%v,want%d", got, observeErr, len(prefix)+want)
	}
	if closeErr := d.Close(); !errors.Is(closeErr, os.ErrClosed) {
		t.Fatalf("native close=%v,want closed cause", closeErr)
	}
	after, err := d.AcceptedBytes()
	if err != nil || after != got {
		t.Fatalf("settlement changed actual acceptance=%v/%v,want%v", after, err, got)
	}
}

// Internal representation pressure is not a claim to have written 8EiB.
func TestAppendAcceptanceSignedSizeBoundaryRefusesBeforeWrite(t *testing.T) {
	t.Parallel()
	file, err := os.Create(t.TempDir() + "/native")
	if err != nil {
		t.Fatal(err)
	}
	initial, err := core.NewByteLength(math.MaxInt64 - 2)
	if err != nil {
		t.Fatal(err)
	}
	d := &AppendDestination{file: file, writer: bufio.NewWriter(file), accepted: initial}
	d.self = d
	defer d.Close()
	n, err := d.Write([]byte("abc"))
	if n != 0 || !errors.Is(err, core.ErrNumericOverflow) {
		t.Fatalf("over-bound write=%d/%v,want zero/overflow", n, err)
	}
	count, err := d.AcceptedBytes()
	if err != nil || count != initial || d.writer.Buffered() != 0 {
		t.Fatalf("overflow changed acceptance=%v/%v,buffered=%d", count, err, d.writer.Buffered())
	}
	n, err = d.Write([]byte("ab"))
	if err != nil || n != 2 {
		t.Fatalf("exact bound write=%d/%v,want2", n, err)
	}
	count, err = d.AcceptedBytes()
	if err != nil || count.Uint64() != math.MaxInt64 {
		t.Fatalf("exact bound acceptance=%v/%v", count, err)
	}
	n, err = d.Write([]byte("c"))
	if n != 0 || !errors.Is(err, core.ErrNumericOverflow) {
		t.Fatalf("past exact bound=%d/%v", n, err)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(file.Name())
	if err != nil || string(body) != "ab" {
		t.Fatalf("representation fixture bytes=%q/%v,want ab", body, err)
	}
}
