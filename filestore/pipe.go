package filestore

import (
	"context"
	"os"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// Pipe is one anonymous OS pipe. The caller owns both real Go handles and
// closes each end when that direction is finished. No worker or buffer is
// added by Filestore; the OS supplies backpressure and Go owns I/O.
type Pipe struct {
	Reader *os.File
	Writer *os.File
}

// Validate rejects missing or aliased endpoint custody. It does not inspect
// current descriptor state: callers may close either end independently.
func (p Pipe) Validate() error {
	if p.Reader == nil || p.Writer == nil || p.Reader == p.Writer {
		return core.ErrFilestoreContract
	}
	return nil
}

// OpenPipe acquires one anonymous pipe after context admission. A refusal
// returns neither endpoint. The returned handles have os.Pipe semantics.
func OpenPipe(ctx context.Context) (Pipe, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return Pipe{}, err
	}
	reader, writer, err := os.Pipe()
	if err != nil {
		return Pipe{}, sourceError(err)
	}
	return Pipe{Reader: reader, Writer: writer}, nil
}

var _ core.Validatable = Pipe{}
