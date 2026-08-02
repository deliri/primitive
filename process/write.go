package process

import (
	"errors"
	"io"
)

// forwardFullWrite is the one package rule for forwarding an admitted output
// prefix. Empty prefixes never reach caller code, and every nonempty write is
// fully accepted or reported as a short write. Policy above this substrate
// decides whether bytes beyond the admitted prefix are rejected or dropped.
func forwardFullWrite(destination io.Writer, retained *uint64, buffer []byte) (int, error) {
	if len(buffer) == 0 {
		return 0, nil
	}
	count, err := destination.Write(buffer)
	if count < 0 || count > len(buffer) {
		return 0, errors.Join(io.ErrShortWrite, err)
	}
	*retained += uint64(count)
	if err != nil {
		return count, err
	}
	if count != len(buffer) {
		return count, io.ErrShortWrite
	}
	return count, nil
}
