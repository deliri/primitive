package filestore

import (
	"context"
	"fmt"
	"os"

	"github.com/deliri/primitive/v2026/core"
)

// FileScopeResult separates consumer meaning from native closure. Its zero
// value is invalid. A valid result proves a close attempt, not successful
// cleanup or consumption of all bytes; callers inspect both error fields.
type FileScopeResult struct {
	operationError error
	cleanupError   error
	completed      bool
}

type fileScopeCallback struct {
	Use         func(context.Context, *os.File) error
	PanicPolicy ScopePanicPolicy
}

func (r FileScopeResult) OperationError() error { return r.operationError }
func (r FileScopeResult) CleanupError() error   { return r.cleanupError }

func (r FileScopeResult) Validate() error {
	if !r.completed {
		return core.ErrFilestoreContract
	}
	return nil
}

// finishFileScope owns the native close on every return path. The real file
// has no second lifecycle model; the result records the observed close attempt.
func finishFileScope(ctx context.Context, file *os.File, callback fileScopeCallback) (result FileScopeResult) {
	defer func() {
		if err := file.Close(); err != nil {
			result.cleanupError = cleanupError(err)
		}
		result.completed = true
	}()
	result.operationError = useFileScope(ctx, file, callback)
	return result
}

func useFileScope(ctx context.Context, file *os.File, callback fileScopeCallback) (resultErr error) {
	if callback.PanicPolicy == ScopePanicPropagate {
		return callback.Use(ctx, file)
	}
	defer func() {
		if recover() != nil {
			resultErr = fmt.Errorf("file scope callback: %w", core.ErrFilestoreCallbackPanic)
		}
	}()
	return callback.Use(ctx, file)
}

var _ core.Validatable = FileScopeResult{}
