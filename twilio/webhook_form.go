package twilio

import (
	"bufio"
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"io"
	"iter"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/lineio"
)

// These immutable coordinates borrow native source bytes; empty names and
// values are legal form facts, so they are not physical RecordRange values.
type webhookByteSpan struct {
	Offset int64
	Bytes  int64
}

type webhookFormField struct {
	Name  webhookByteSpan
	Value webhookByteSpan
}

type webhookDecodedReader struct{ reader *bufio.Reader }

func (r webhookDecodedReader) ReadByte() (byte, error) {
	value, err := r.reader.ReadByte()
	if err != nil {
		return 0, err
	}
	switch value {
	case core.TwilioWebhookInvalidFieldSeparator:
		return 0, core.ErrTwilioBinding
	case core.TwilioWebhookSpaceEncoding:
		return core.TwilioWebhookSpace, nil
	case core.TwilioWebhookEscape:
		var encoded [2]byte
		if _, err := io.ReadFull(r.reader, encoded[:]); err != nil {
			return 0, errors.Join(core.ErrTwilioBinding, err)
		}
		var decoded [1]byte
		if _, err := hex.Decode(decoded[:], encoded[:]); err != nil {
			return 0, errors.Join(core.ErrTwilioBinding, err)
		}
		return decoded[0], nil
	default:
		return value, nil
	}
}

func decodedWebhookSpan(source io.ReaderAt, span webhookByteSpan) webhookDecodedReader {
	return webhookDecodedReader{reader: bufio.NewReaderSize(io.NewSectionReader(source, span.Offset, span.Bytes), exchange.TransferBufferBytes)}
}

func compareWebhookSpans(ctx context.Context, left io.ReaderAt, a webhookByteSpan, right io.ReaderAt, b webhookByteSpan) (core.Comparison, error) {
	// Comparison retains a small native read buffer even for arbitrary key
	// extents. Value hashing uses the larger transfer buffer separately.
	x := webhookDecodedReader{reader: bufio.NewReaderSize(io.NewSectionReader(left, a.Offset, a.Bytes), 64)}
	y := webhookDecodedReader{reader: bufio.NewReaderSize(io.NewSectionReader(right, b.Offset, b.Bytes), 64)}
	for {
		if err := contextstate.Validate(ctx); err != nil {
			return core.ComparisonUnknown, err
		}
		first, firstErr := x.ReadByte()
		second, secondErr := y.ReadByte()
		if firstErr != nil && firstErr != io.EOF || secondErr != nil && secondErr != io.EOF {
			return core.ComparisonUnknown, errors.Join(firstErr, secondErr)
		}
		switch {
		case firstErr == io.EOF && secondErr == io.EOF:
			return core.ComparisonEqual, nil
		case firstErr == io.EOF:
			return core.ComparisonLess, nil
		case secondErr == io.EOF:
			return core.ComparisonGreater, nil
		case first < second:
			return core.ComparisonLess, nil
		case first > second:
			return core.ComparisonGreater, nil
		}
	}
}

func walkWebhookFields(ctx context.Context, source io.Reader) iter.Seq2[lineio.RecordRange, error] {
	return func(yield func(lineio.RecordRange, error) bool) {
		reader := bufio.NewReaderSize(source, exchange.TransferBufferBytes)
		var start, offset int64
		for {
			if err := contextstate.Validate(ctx); err != nil {
				yield(lineio.RecordRange{}, err)
				return
			}
			fragment, err := reader.ReadSlice(core.TwilioWebhookFieldSeparator)
			if bytes.IndexByte(fragment, core.TwilioWebhookInvalidFieldSeparator) >= 0 {
				yield(lineio.RecordRange{}, core.ErrTwilioBinding)
				return
			}
			if err != nil && err != bufio.ErrBufferFull && err != io.EOF {
				yield(lineio.RecordRange{}, err)
				return
			}
			previous := offset
			offset += int64(len(fragment))
			if offset < previous {
				yield(lineio.RecordRange{}, core.ErrNumericOverflow)
				return
			}
			if err == bufio.ErrBufferFull {
				continue
			}
			end := offset
			if err == nil {
				end--
			}
			if end > start {
				extent, extentErr := core.NewByteLength(uint64(end - start))
				record := lineio.RecordRange{Offset: lineio.SourceByteOffset(start), Bytes: extent, Framing: lineio.RecordFramingOpaque}
				if extentErr != nil || !yield(record, nil) {
					if extentErr != nil {
						yield(lineio.RecordRange{}, extentErr)
					}
					return
				}
			}
			start = offset
			if err == io.EOF {
				return
			}
		}
	}
}

func splitWebhookField(source io.ReaderAt, record lineio.RecordRange) (webhookFormField, error) {
	if err := record.Validate(); err != nil {
		return webhookFormField{}, err
	}
	extent, err := record.Bytes.Int64()
	if err != nil {
		return webhookFormField{}, err
	}
	offset := int64(record.Offset)
	reader := bufio.NewReaderSize(io.NewSectionReader(source, offset, extent), 64)
	var nameBytes int64
	for {
		fragment, err := reader.ReadSlice(core.TwilioWebhookValueSeparator)
		nameBytes += int64(len(fragment))
		switch err {
		case nil:
			return webhookFormField{Name: webhookByteSpan{Offset: offset, Bytes: nameBytes - 1}, Value: webhookByteSpan{Offset: offset + nameBytes, Bytes: extent - nameBytes}}, nil
		case bufio.ErrBufferFull:
			continue
		case io.EOF:
			if nameBytes != extent {
				return webhookFormField{}, errors.Join(core.ErrTwilioBinding, io.ErrUnexpectedEOF)
			}
			return webhookFormField{Name: webhookByteSpan{Offset: offset, Bytes: extent}, Value: webhookByteSpan{Offset: offset + extent}}, nil
		default:
			return webhookFormField{}, err
		}
	}
}

func writeDecodedWebhookSpan(ctx context.Context, source io.ReaderAt, span webhookByteSpan, destination io.Writer) error {
	reader := decodedWebhookSpan(source, span)
	var buffer [exchange.TransferBufferBytes]byte
	for {
		if err := contextstate.Validate(ctx); err != nil {
			return err
		}
		count := 0
		for count < len(buffer) {
			value, err := reader.ReadByte()
			if err != nil && err != io.EOF {
				return err
			}
			if err == io.EOF {
				if count == 0 {
					return nil
				}
				break
			}
			buffer[count] = value
			count++
		}
		written, err := destination.Write(buffer[:count])
		if err != nil {
			return err
		}
		if written != count {
			return io.ErrShortWrite
		}
	}
}

func writeSortedWebhookForm(ctx context.Context, parent core.AbsolutePath, source lineio.RecordSource, extent int64, destination io.Writer) error {
	var previous webhookByteSpan
	havePrevious := false
	_, err := filestore.SortRecordIndexStream(ctx, filestore.RecordRangeIndexStreamRequest{
		Records: filestore.RecordRangeStreamRequest{
			Parent: parent, Source: source,
			Compare: func(ctx context.Context, source lineio.RecordSource, a, b lineio.RecordRange) (core.Comparison, error) {
				left, leftErr := splitWebhookField(source, a)
				right, rightErr := splitWebhookField(source, b)
				if err := errors.Join(leftErr, rightErr); err != nil {
					return core.ComparisonUnknown, err
				}
				return compareWebhookSpans(ctx, source, left.Name, source, right.Name)
			},
			Visit: func(record lineio.RecordRange) error {
				field, err := splitWebhookField(source, record)
				if err != nil {
					return err
				}
				if havePrevious {
					order, err := compareWebhookSpans(ctx, source, previous, source, field.Name)
					if err != nil || order != core.ComparisonLess {
						return errors.Join(core.ErrTwilioVerification, err)
					}
				}
				if err := writeDecodedWebhookSpan(ctx, source, field.Name, destination); err != nil {
					return err
				}
				if err := writeDecodedWebhookSpan(ctx, source, field.Value, destination); err != nil {
					return err
				}
				previous, havePrevious = field.Name, true
				return nil
			},
		},
		Index: func(ctx context.Context, destination io.Writer) error {
			for record, err := range walkWebhookFields(ctx, io.NewSectionReader(source, 0, extent)) {
				if err != nil {
					return err
				}
				if err := filestore.WriteRecordRangeIndex(destination, record); err != nil {
					return err
				}
			}
			return nil
		},
	})
	return err
}

func (webhookByteSpan) twilioInternalFlow()      {}
func (webhookFormField) twilioInternalFlow()     {}
func (webhookDecodedReader) twilioInternalFlow() {}

var _ internalFlow = webhookByteSpan{}
var _ internalFlow = webhookFormField{}
var _ internalFlow = webhookDecodedReader{}
