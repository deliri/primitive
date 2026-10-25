package runprotocol

import (
	"context"
	"errors"
	"io"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/jsonio"
)

// TestResultStreamRequest lends one receipt and the result object stream its
// Results reference names. The source stays caller-owned.
type TestResultStreamRequest struct {
	Source  io.Reader
	Receipt RunReceipt
}

func (r TestResultStreamRequest) Validate() error {
	if core.ReaderIsNil(r.Source) {
		return contractError(errors.New("run protocol result stream is missing"))
	}
	if err := r.Receipt.Validate(); err != nil {
		return err
	}
	if r.Receipt.Results == nil {
		return contractError(errors.New("run receipt names no result stream"))
	}
	return nil
}

// VerifyTestResults reads a receipt's result stream forward once: every
// TestResult validates, ordinals run exactly 1 through n, n is the
// receipt's ResultCount, and the stream's exact bytes match the receipt's
// Results size and digest. Filestore owns exact content verification, the native
// pipe, backpressure and the joined producer lifetime; JSONIO and Go own parsing.
// One record is retained, not a result population. Its largest scalar and Go's
// parser buffers may grow with that record. The source owns interruptible reads.
// Consumers verify before treating results as proof; this is not acceptance.
// FuzzVerifyTestResultsSemanticClosure binds this stream ingress to exact
// byte/digest/count agreement and independently decoded ordinal observations.
func VerifyTestResults(ctx context.Context, request TestResultStreamRequest) error {
	if err := request.Validate(); err != nil {
		return err
	}
	expected := *request.Receipt.Results
	declared, err := expected.Bytes.Uint64()
	if err != nil {
		return contractError(err)
	}
	extent, err := core.NewByteLength(declared)
	if err != nil {
		return contractError(err)
	}
	content := filestore.ContentIndexEntry{Digest: expected.Digest, Extent: extent}
	transfer, operationErr := filestore.TransferPipe(ctx, filestore.PipeTransferRequest{
		Produce: func(ctx context.Context, destination io.Writer) error {
			_, copyErr := filestore.CopyContent(ctx, filestore.CopyContentRequest{Source: request.Source, Destination: destination, Expected: &content})
			if copyErr == nil {
				return nil
			}
			if errors.Is(copyErr, core.ErrFilestoreContract) || errors.Is(copyErr, core.ErrFilestoreSize) || errors.Is(copyErr, core.ErrFilestoreSource) && errors.Is(copyErr, io.ErrUnexpectedEOF) {
				return conflictError(copyErr)
			}
			return contractError(copyErr)
		},
		Consume: func(ctx context.Context, source io.Reader) error {
			return consumeTestResults(ctx, source, request.Receipt.ResultCount)
		},
	})
	resultErr := errors.Join(contractJoin(operationErr, transfer.Validate(), transfer.CleanupError), transfer.ProducerError, transfer.ConsumerError)
	if resultErr != nil && !errors.Is(resultErr, core.ErrRunProtocolContract) && !errors.Is(resultErr, core.ErrRunProtocolConflict) {
		return contractError(resultErr)
	}
	return resultErr
}

func consumeTestResults(ctx context.Context, source io.Reader, expectedCount uint64) error {
	var ordinal uint64
	for result, err := range jsonio.Objects[TestResult](ctx, jsonio.ObjectSourceRequest{Source: source, Members: jsonio.MembersDeclaredOnly}) {
		if err != nil {
			return errors.Join(core.ErrRunProtocolContract, err)
		}
		if ordinal >= expectedCount || result.Ordinal != ordinal+1 {
			return conflictError(errors.New("run protocol result ordinals are not exactly 1 through the receipt's count"))
		}
		ordinal++ // The prior guard proves this increment cannot wrap.
	}
	if ordinal != expectedCount {
		return conflictError(errors.New("run protocol result count does not match its receipt"))
	}
	return nil
}
