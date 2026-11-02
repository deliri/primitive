package process_test

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/deliri/primitive/v2026/process"
)

func BenchmarkExecutionScopeVisitOwnedChild(b *testing.B) {
	b.ReportAllocs()
	ctx, cancel := context.WithCancel(b.Context())
	defer cancel()
	reader, writer := io.Pipe()
	defer reader.Close()
	defer writer.Close()
	var scope process.ExecutionScope
	request := processRequest(b, "copy", process.Streams{Stdin: reader, Stdout: io.Discard, Stderr: io.Discard})
	execution, err := scope.Begin(ctx, request)
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() {
		_ = writer.Close()
		_ = reader.Close()
		cancel()
		if scope.ActiveCount() != 0 {
			if _, err := scope.Wait(process.ExecutionWaitRequest{Execution: execution}); err != nil && !errors.Is(err, context.Canceled) {
				b.Error(err)
			}
		}
	})
	visits := 0
	borrowed := process.ActiveExecutionRequest{Visit: func(active *process.Execution) error {
		if active != execution {
			b.Fatalf("visitor borrowed %p, want %p", active, execution)
		}
		visits++
		return nil
	}}
	for b.Loop() {
		if err := scope.VisitActive(ctx, borrowed); err != nil {
			b.Fatal(err)
		}
	}
	if visits != b.N {
		b.Fatalf("visits=%d,want %d native capability visits", visits, b.N)
	}
	if err := writer.Close(); err != nil {
		b.Fatal(err)
	}
	result, err := scope.Wait(process.ExecutionWaitRequest{Execution: execution})
	if err := errors.Join(err, result.Validate()); err != nil || scope.ActiveCount() != 0 {
		b.Fatalf("wait=%v active=%d,want completed native custody", err, scope.ActiveCount())
	}
}
