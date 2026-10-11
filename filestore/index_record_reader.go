package filestore

import (
	"errors"
	"github.com/deliri/primitive/v2026/core"
	"io"
)

// readIndexRecord reads one fixed tuple through the borrowed Go reader. It
// retains an accompanying native refusal even at the last required byte, which
// io.ReadFull otherwise suppresses. There is no file-lookalike or read cache.
func readIndexRecord(source io.Reader, data []byte) error {
	observed := 0
	for observed < len(data) {
		n, err := source.Read(data[observed:])
		if n < 0 || n > len(data)-observed {
			return errors.Join(core.ErrFilestoreContract, io.ErrShortBuffer, err)
		}
		observed += n
		if err == io.EOF {
			if observed == 0 {
				return io.EOF
			}
			if observed == len(data) {
				return nil
			}
			return errors.Join(core.ErrFilestoreContract, io.ErrUnexpectedEOF)
		}
		if err != nil {
			return errors.Join(core.ErrFilestoreContract, err)
		}
	}
	return nil
}
