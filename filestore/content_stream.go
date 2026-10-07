package filestore

import (
	"context"
	"errors"
	"io"
	"iter"
	"math"
	"os"
	"path/filepath"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// ContentStreamRequest borrows a synchronous source and destination. Primitive
// owns its disposable sort files below Parent. The source determines record
// meaning; this capability observes and orders exact digest/extent records.
type ContentStreamRequest struct {
	Parent      core.AbsolutePath
	Source      iter.Seq2[ContentIndexEntry, error]
	Destination io.Writer
}

func (r ContentStreamRequest) Validate() error {
	if err := r.Parent.Validate(); err != nil {
		return errors.Join(core.ErrFilestoreContract, err)
	}
	if r.Source == nil || core.WriterIsNil(r.Destination) {
		return core.ErrFilestoreContract
	}
	return nil
}

// ContentStreamSummary distinguishes observed records from sorted unique
// records. Equality policy belongs to the caller. A failed operation returns
// no summary, including when native cleanup fails.
type ContentStreamSummary struct {
	Observed uint64
	Unique   ContentIndexSummary
}

func (s ContentStreamSummary) Validate() error {
	if err := s.Unique.Validate(); err != nil {
		return err
	}
	if s.Unique.Entries > s.Observed {
		return core.ErrFilestoreContract
	}
	return nil
}

// SortContentStream retains only fixed sort buffers and one source record.
// There is no input cardinality ceiling. Destination bytes are provisional
// until the source, sort, and native cleanup have all succeeded.
func SortContentStream(ctx context.Context, request ContentStreamRequest) (summary ContentStreamSummary, resultErr error) {
	if err := errors.Join(contextstate.Validate(ctx), request.Validate()); err != nil {
		return ContentStreamSummary{}, err
	}
	defer func() {
		if resultErr != nil {
			summary = ContentStreamSummary{}
		}
	}()
	root, err := OpenRoot(ctx, request.Parent)
	if err != nil {
		return ContentStreamSummary{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, root.Close()) }()
	directory, err := os.MkdirTemp(request.Parent.String(), "primitive-content-sort-")
	if err != nil {
		return ContentStreamSummary{}, activationError(err)
	}
	defer func() {
		path, err := core.ParseRelativePath(filepath.Base(directory))
		if err != nil {
			resultErr = errors.Join(resultErr, err)
			return
		}
		resultErr = errors.Join(resultErr, RemoveTree(context.WithoutCancel(ctx), TreeRemovalRequest{Location: Location{Root: root, Path: path}}))
	}()
	var files [2]*os.File
	for index, name := range [...]string{"source", "merge"} {
		file, err := os.OpenFile(filepath.Join(directory, name), os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
		if err != nil {
			return ContentStreamSummary{}, activationError(err)
		}
		defer func() {
			if err := file.Close(); err != nil {
				resultErr = errors.Join(resultErr, cleanupError(err))
			}
		}()
		files[index] = file
	}
	observed, err := writeContentStream(ctx, files[0], request.Source)
	if err != nil {
		return ContentStreamSummary{}, err
	}
	unique, err := SortContentIndex(ctx, ContentSortRequest{Files: files, Destination: request.Destination})
	if err != nil {
		return ContentStreamSummary{}, err
	}
	summary = ContentStreamSummary{Observed: observed, Unique: unique}
	return summary, summary.Validate()
}

func writeContentStream(ctx context.Context, destination io.Writer, source iter.Seq2[ContentIndexEntry, error]) (uint64, error) {
	var observed uint64
	for record, err := range source {
		if err := errors.Join(err, contextstate.Validate(ctx)); err != nil {
			return 0, err
		}
		if observed == math.MaxUint64 {
			return 0, core.ErrNumericOverflow
		}
		if err := WriteContentIndexEntry(destination, record); err != nil {
			return 0, err
		}
		observed++
	}
	if err := contextstate.Validate(ctx); err != nil {
		return 0, err
	}
	return observed, nil
}
