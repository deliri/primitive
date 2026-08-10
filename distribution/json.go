package distribution

import (
	"errors"
	"io"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	requestPayloadJSONMaximumBytes         = 96 << 10
	requestDocumentJSONMaximumBytes        = 128 << 10
	responsePayloadJSONMaximumBytes        = 128 << 10
	responseDocumentJSONMaximumBytes       = 256 << 10
	publicationGrantJSONMaximumBytes       = 256 << 10
	publicationCompletionMaximumBytes      = 128 << 10
	documentCommitmentFrameSeparator  byte = 0
)

func decodeStrict[T any](data []byte, maximum uint64) (T, error) {
	var zero T
	limit, err := core.NewByteCount(maximum)
	if err != nil {
		return zero, jsonError(err)
	}
	limits := core.DefaultStrictJSONLimits()
	limits.DocumentMaximumBytes = limit
	decoded, err := core.DecodeStrictJSONStructure[T](data, limits)
	if err != nil {
		return zero, jsonError(err)
	}
	return decoded, nil
}

func writeCanonical(destination io.Writer, encoded []byte) error {
	if destination == nil {
		return contractError(errors.New("canonical destination is nil"))
	}
	written, err := destination.Write(encoded)
	if err != nil {
		return contractError(err)
	}
	if written != len(encoded) {
		return contractError(io.ErrShortWrite)
	}
	return nil
}

func validateLifetime(issued, expires temporal.Instant) error {
	if err := errors.Join(issued.Validate(), expires.Validate()); err != nil {
		return contractError(err)
	}
	order, err := issued.Compare(expires)
	if err != nil || order != core.ComparisonLess {
		return contractError(errors.New("distribution lifetime is not strictly ordered"), err)
	}
	return nil
}

func validateObservedLifetime(issued, expires, observed temporal.Instant) error {
	if err := errors.Join(validateLifetime(issued, expires), observed.Validate()); err != nil {
		return bindingError(err)
	}
	from, fromErr := observed.Compare(issued)
	until, untilErr := observed.Compare(expires)
	if errors.Join(fromErr, untilErr) != nil || from == core.ComparisonLess || until != core.ComparisonLess {
		return bindingError(errors.New("distribution document is outside its signed lifetime"), fromErr, untilErr)
	}
	return nil
}
