package runprotocol

import (
	"context"
	"errors"
	"io"
	"iter"
	"math"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/jsonio"
)

type RunFactWriteRequest struct {
	Destination io.Writer
	Facts       iter.Seq2[RunFact, error]
}

func (r RunFactWriteRequest) Validate() error {
	if core.WriterIsNil(r.Destination) || r.Facts == nil {
		return core.ErrRunProtocolContract
	}
	return nil
}

type RunFactStreamRequest struct {
	Source  io.Reader
	Receipt RunReceipt
}

func (r RunFactStreamRequest) Validate() error {
	if core.ReaderIsNil(r.Source) {
		return core.ErrRunProtocolContract
	}
	return r.Receipt.Validate()
}

// WriteRunFacts emits deterministic typed JSON objects with one LF per record.
// Count, extent and digest cover precisely these compact bytes, not a header or
// any raw native output. A failed write returns no usable sequence proof.
func WriteRunFacts(ctx context.Context, request RunFactWriteRequest) (RunFactSequence, error) {
	if err := contractJoin(runReceiptContextError(ctx), request.Validate()); err != nil {
		return RunFactSequence{}, err
	}
	digest := core.NewDigestWriter()
	encoder, err := jsonio.NewObjectEncoder[RunFact](jsonio.ObjectDestinationRequest{Destination: io.MultiWriter(request.Destination, digest)})
	if err != nil {
		return RunFactSequence{}, contractError(err)
	}
	var count uint64
	for fact, err := range request.Facts {
		if err != nil {
			return RunFactSequence{}, contractError(err)
		}
		if count == math.MaxUint64 || fact.Ordinal != count+1 {
			return RunFactSequence{}, conflictError(nil)
		}
		if err := encoder.Encode(ctx, fact); err != nil {
			return RunFactSequence{}, contractError(err)
		}
		count++
	}
	if err := runReceiptContextError(ctx); err != nil {
		return RunFactSequence{}, contractError(err)
	}
	sha, extent, err := digest.Seal()
	if err != nil {
		return RunFactSequence{}, contractError(err)
	}
	result := RunFactSequence{Count: count, Bytes: extent, Digest: sha}
	if err := result.Validate(); err != nil {
		return RunFactSequence{}, err
	}
	return result, nil
}

// RunFacts borrows an interruptible source and keeps only the current compact
// record. A prefix is provisional: only consuming EOF without error verifies
// the sequence, canonical bytes and summary. Breaking early verifies nothing.
func RunFacts(ctx context.Context, request RunFactStreamRequest) iter.Seq2[RunFact, error] {
	return func(yield func(RunFact, error) bool) {
		fail := func(err error) { yield(RunFact{}, contractError(err)) }
		if err := contractJoin(runReceiptContextError(ctx), request.Validate()); err != nil {
			fail(err)
			return
		}
		length, err := request.Receipt.Facts.Bytes.Int64()
		if err != nil {
			fail(err)
			return
		}
		bounded := &io.LimitedReader{R: request.Source, N: length}
		raw := core.NewDigestWriter()
		canonical := core.NewDigestWriter()
		encoder, err := jsonio.NewObjectEncoder[RunFact](jsonio.ObjectDestinationRequest{Destination: canonical})
		if err != nil {
			fail(err)
			return
		}
		var count uint64
		var totals runFactTotals
		for fact, err := range jsonio.Objects[RunFact](ctx, jsonio.ObjectSourceRequest{Source: io.TeeReader(bounded, raw), Members: jsonio.MembersDeclaredOnly}) {
			if err != nil {
				fail(err)
				return
			}
			if count >= request.Receipt.Facts.Count || fact.Ordinal != count+1 {
				fail(conflictError(nil))
				return
			}
			if err := totals.add(fact, request.Receipt); err != nil {
				fail(err)
				return
			}
			if err := encoder.Encode(ctx, fact); err != nil {
				fail(err)
				return
			}
			count++
			if !yield(fact, nil) {
				return
			}
		}
		if err := runReceiptContextError(ctx); err != nil {
			fail(err)
			return
		}
		if bounded.N != 0 || count != request.Receipt.Facts.Count {
			fail(conflictError(nil))
			return
		}
		var extra [1]byte
		n, readErr := request.Source.Read(extra[:])
		if err := runReceiptContextError(ctx); err != nil {
			fail(errors.Join(err, readErr))
			return
		}
		if n != 0 {
			fail(conflictError(readErr))
			return
		}
		if readErr != io.EOF {
			if readErr == nil {
				readErr = io.ErrNoProgress
			}
			fail(readErr)
			return
		}
		for _, writer := range [...]*core.DigestWriter{raw, canonical} {
			sha, extent, sealErr := writer.Seal()
			if sealErr != nil {
				fail(sealErr)
				return
			}
			if sha != request.Receipt.Facts.Digest || extent != request.Receipt.Facts.Bytes {
				fail(conflictError(nil))
				return
			}
		}
		if err := totals.validate(request.Receipt); err != nil {
			fail(err)
		}
	}
}

// VerifyRunFacts consumes the entire compact stream; it never reruns native
// execution or consults raw artifacts.
func VerifyRunFacts(ctx context.Context, request RunFactStreamRequest) error {
	if err := contractJoin(runReceiptContextError(ctx), request.Validate()); err != nil {
		return err
	}
	content := filestore.ContentIndexEntry{Digest: request.Receipt.Facts.Digest, Extent: request.Receipt.Facts.Bytes}
	transfer, operationErr := filestore.TransferPipe(ctx, filestore.PipeTransferRequest{
		Produce: func(ctx context.Context, destination io.Writer) error {
			_, err := filestore.CopyContent(ctx, filestore.CopyContentRequest{Source: request.Source, Destination: destination, Expected: &content})
			if err == nil {
				return nil
			}
			if errors.Is(err, core.ErrFilestoreContract) || errors.Is(err, core.ErrFilestoreSize) || errors.Is(err, core.ErrFilestoreSource) && errors.Is(err, io.ErrUnexpectedEOF) {
				return conflictError(err)
			}
			return contractError(err)
		},
		Consume: func(ctx context.Context, source io.Reader) error {
			for _, err := range RunFacts(ctx, RunFactStreamRequest{Source: source, Receipt: request.Receipt}) {
				if err != nil {
					return err
				}
			}
			return nil
		},
	})
	return contractJoin(operationErr, transfer.Validate(), transfer.CleanupError, transfer.ProducerError, transfer.ConsumerError)
}

type runFactTotals struct {
	tests               RunCounts
	packages            RunCounts
	benchmarks          RunBenchmarkCounts
	invocations         uint64
	executions          uint64
	summaryExitObserved bool
	fuzzPassed          uint64
	buildFailed         bool
}

func (t *runFactTotals) add(f RunFact, receipt RunReceipt) error {
	switch f.Kind {
	case RunFactInvocation:
		if t.invocations == math.MaxUint64 {
			return conflictError(nil)
		}
		t.invocations++
		if receipt.Source.Captured == nil || f.Invocation.Source != receipt.Source.Captured.Digest {
			return conflictError(nil)
		}
	case RunFactExecution:
		if t.executions == math.MaxUint64 {
			return conflictError(nil)
		}
		t.executions++
		if receipt.Summary.ExitCode != nil {
			if *receipt.Summary.ExitCode == core.ProcessExitCodeSuccess && f.Execution.ExitCode != core.ProcessExitCodeSuccess {
				return conflictError(nil)
			}
			t.summaryExitObserved = t.summaryExitObserved || f.Execution.ExitCode == *receipt.Summary.ExitCode
		}
	case RunFactResult:
		r := f.Result
		switch r.Kind {
		case RunResultPackage:
			if *r.Outcome == OutcomeSetupFailed {
				t.buildFailed = true
				return nil
			}
			return addRunTerminal(&t.packages, *r.Outcome)
		case RunResultTest:
			return addRunTerminal(&t.tests, *r.Outcome)
		case RunResultFuzz:
			if receipt.Kind != RunKindFuzz {
				return conflictError(nil)
			}
			if *r.Outcome == OutcomePassed {
				if t.fuzzPassed == math.MaxUint64 {
					return conflictError(nil)
				}
				t.fuzzPassed++
			}
			return addRunTerminal(&t.tests, *r.Outcome)
		case RunResultBenchmark:
			if receipt.Kind != RunKindBenchmark || t.benchmarks.Rows == math.MaxUint64 {
				return conflictError(nil)
			}
			t.benchmarks.Rows++
			if r.Benchmark.Kind == GoBenchmarkRowMeasured {
				t.benchmarks.Measured++
			}
		default:
			return core.ErrRunProtocolContract
		}
	default:
		return core.ErrRunProtocolContract
	}
	return nil
}
func addRunTerminal(counts *RunCounts, outcome Outcome) error {
	var count *uint64
	switch outcome {
	case OutcomePassed:
		count = &counts.Passed
	case OutcomeFailed:
		count = &counts.Failed
	case OutcomeSkipped:
		count = &counts.Skipped
	default:
		return core.ErrRunProtocolContract
	}
	if *count == math.MaxUint64 {
		return conflictError(nil)
	}
	*count++
	return nil
}
func (t runFactTotals) validate(r RunReceipt) error {
	s := r.Summary
	if t.tests.Passed != s.Tests.Passed || t.tests.Failed != s.Tests.Failed || t.tests.Skipped != s.Tests.Skipped || t.packages.Passed != s.Packages.Passed || t.packages.Failed != s.Packages.Failed || t.packages.Skipped != s.Packages.Skipped || t.buildFailed && !s.BuildFailed {
		return conflictError(nil)
	}
	if t.executions > 0 && t.invocations == 0 || s.ExitCode != nil && (t.executions == 0 || !t.summaryExitObserved) {
		return conflictError(nil)
	}
	if r.Outcome == OutcomePassed && r.Kind == RunKindFuzz && t.fuzzPassed == 0 {
		return conflictError(nil)
	}
	if s.Benchmarks == nil {
		if t.benchmarks.Rows != 0 {
			return conflictError(nil)
		}
	} else if t.benchmarks != *s.Benchmarks {
		return conflictError(nil)
	}
	return nil
}

// Preserve a terminal caller cause using the existing native context-state boundary.
func runReceiptContextError(ctx context.Context) (err error) {
	err = contextstate.Validate(ctx)
	if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	defer func() { _ = recover() }()
	return errors.Join(err, context.Cause(ctx))
}
