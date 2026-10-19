package filestore

import (
	"bufio"
	"errors"
	"github.com/deliri/primitive/v2026/core"
	"io"
)

// bufferedIndexRecord borrows one complete fixed tuple from Go's buffered reader
// without copying it through a heap-escaping caller array. ok is false when the
// reader's window cannot hold the tuple; the caller then uses readIndexRecord.
// Go's Peek owns the fill, consecutive-empty-read and native-error loop, so the
// refusals match readIndexRecord: clean EOF before any byte is io.EOF, a short
// tuple is ErrUnexpectedEOF and every other native refusal is a Source failure.
// The borrowed bytes are valid until the next read; the caller discards them.
func bufferedIndexRecord(source io.Reader, size int) (data []byte, ok bool, err error) {
	reader, buffered := source.(*bufio.Reader)
	if !buffered || reader.Size() < size {
		return nil, false, nil
	}
	data, err = reader.Peek(size)
	if len(data) == size {
		return data, true, nil
	}
	if err == io.EOF && len(data) == 0 {
		return nil, true, io.EOF
	}
	if err == io.EOF || err == nil {
		return nil, true, sourceError(io.ErrUnexpectedEOF)
	}
	return nil, true, sourceError(err)
}

// appendIndexRecordScratch lends Go's unused buffered-writer window so a fixed
// tuple is encoded in place. Other writers receive nil and append allocates.
func appendIndexRecordScratch(destination io.Writer) []byte {
	if writer, ok := destination.(*bufio.Writer); ok {
		return writer.AvailableBuffer()
	}
	return nil
}

// readIndexRecord reads one fixed tuple through the borrowed Go reader. It
// retains an accompanying native refusal even at the last required byte, which
// io.ReadFull otherwise suppresses. There is no file-lookalike or read cache.
func readIndexRecord(source io.Reader, data []byte) error {
	observed := 0
	emptyReads := 0
	for observed < len(data) {
		n, err := source.Read(data[observed:])
		if n < 0 || n > len(data)-observed {
			return errors.Join(core.ErrFilestoreContract, io.ErrShortBuffer, err)
		}
		observed += n
		if n > 0 {
			emptyReads = 0
		} else if err == nil {
			emptyReads++
			if emptyReads >= core.ReaderConsecutiveEmptyReadMaximum {
				return sourceError(io.ErrNoProgress)
			}
		}
		if err == io.EOF {
			if observed == 0 {
				return io.EOF
			}
			if observed == len(data) {
				return nil
			}
			return sourceError(io.ErrUnexpectedEOF)
		}
		if err != nil {
			return sourceError(err)
		}
	}
	return nil
}
