package filestore

import (
	"context"
	"errors"
	"io"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// CopyContentRequest borrows two Go streams for one synchronous operation.
// Expected, when present, is an exact content agreement, never a volume quota.
// Buffer is optional caller-owned scratch, as in ReadRequest and StageRequest.
type CopyContentRequest struct {
	Source      io.Reader
	Destination io.Writer
	Expected    *ContentIndexEntry
	Buffer      []byte
}

func (r CopyContentRequest) Validate() error {
	if core.ReaderIsNil(r.Source) || core.WriterIsNil(r.Destination) {
		return core.ErrFilestoreContract
	}
	if r.Expected != nil {
		return r.Expected.Validate()
	}
	return nil
}

// CopyContent streams bytes and observes their exact SHA-256 and extent using
// the same validated Go copy loop as file reads and staged writes. Empty is a
// real observation. Streams and cleanup remain caller-owned. Any failure returns
// no content proof; the destination may contain provisional partial bytes.
func CopyContent(ctx context.Context, request CopyContentRequest) (ContentIndexEntry, error) {
	if err := errors.Join(contextstate.Validate(ctx), request.Validate()); err != nil {
		return ContentIndexEntry{}, err
	}
	digest := core.NewDigestWriter()
	stream := streamReader{ctx: ctx, source: request.Source}
	copy := streamCopyRequest{ctx: ctx, source: &stream, destination: io.MultiWriter(request.Destination, digest), buffer: request.Buffer, kind: streamDestinationCaller}
	var expected ContentIndexEntry
	if request.Expected != nil {
		expected = *request.Expected
		extent, err := expected.Extent.Int64()
		if err != nil {
			return ContentIndexEntry{}, err
		}
		copy.source = &io.LimitedReader{R: &stream, N: extent}
		copy.extentKnown, copy.knownExtent = true, expected.Extent.Uint64()
	}
	length, err := copyStream(copy)
	if err != nil {
		return ContentIndexEntry{}, err
	}
	if err := verifyContentSourceEnd(&stream, request.Expected); err != nil {
		return ContentIndexEntry{}, err
	}
	sum, measured, err := digest.Seal()
	if err != nil {
		return ContentIndexEntry{}, err
	}
	content := ContentIndexEntry{Digest: sum, Extent: measured}
	if !contentCopyMatches(content, expected, length, request.Expected != nil) {
		return ContentIndexEntry{}, core.ErrFilestoreContract
	}
	if err := errors.Join(content.Validate(), contextstate.Validate(ctx)); err != nil {
		return ContentIndexEntry{}, err
	}
	return content, nil
}

func verifyContentSourceEnd(source io.Reader, expected *ContentIndexEntry) error {
	if expected == nil {
		return nil
	}
	return contentSourceEnded(source)
}

func contentSourceEnded(source io.Reader) error {
	var extra [1]byte
	count, err := io.ReadFull(source, extra[:])
	if count != 0 {
		return errors.Join(core.ErrFilestoreSize, err)
	}
	// witness:waiver doctrine/error/sentinel_compare -- Exact EOF is required; errors.Is would erase a joined source failure.
	if err != io.EOF {
		return errors.Join(core.ErrFilestoreSource, err)
	}
	return nil
}

var _ core.Validatable = CopyContentRequest{}

func contentCopyMatches(content, expected ContentIndexEntry, length core.ByteLength, agreed bool) bool {
	return content.Extent == length && (!agreed || content == expected)
}
