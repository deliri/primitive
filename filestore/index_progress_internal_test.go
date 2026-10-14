package filestore

import (
	"errors"
	"io"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

type indexNoProgressSource struct{ calls int }

func (r *indexNoProgressSource) Read([]byte) (int, error) {
	r.calls++
	if r.calls > core.ReaderConsecutiveEmptyReadMaximum {
		return 0, io.ErrClosedPipe
	}
	return 0, nil
}

func TestIndexRecordStopsNoProgressBeforeReadingInventedLaterFailure(t *testing.T) {
	t.Parallel()
	source := &indexNoProgressSource{}
	var record [32]byte
	got := readIndexRecord(source, record[:])
	if !errors.Is(got, io.ErrNoProgress) || errors.Is(got, io.ErrClosedPipe) || source.calls != core.ReaderConsecutiveEmptyReadMaximum {
		t.Fatalf("nonprogressing index source = %v after %d reads, want native no-progress at %d reads without an unobserved later failure", got, source.calls, core.ReaderConsecutiveEmptyReadMaximum)
	}
}
