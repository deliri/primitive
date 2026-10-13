package lineio_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lineio"
)

func FuzzRecordFieldsMatchIndependentStringFields(f *testing.F) {
	for _, text := range []string{"go test -run=FuzzSelected/seed", "# example.com/package", "\u0085\xff λ\u2003token", "", "\x00\tfield\nlast", strings.Repeat("x", 4097) + " next"} {
		f.Add([]byte(text))
	}
	f.Fuzz(func(t *testing.T, body []byte) {
		// Trailing whitespace makes the opaque source range nonempty without
		// changing fields, including the neutral empty-input representation.
		sourceBytes := append(append([]byte{}, body...), ' ')
		length, err := core.NewByteLength(uint64(len(sourceBytes)))
		if err != nil {
			t.Fatal(err)
		}
		window, err := core.NewByteCount(7)
		if err != nil {
			t.Fatal(err)
		}
		source := bytes.NewReader(sourceBytes)
		request := lineio.RecordFragmentRequest{Source: source, Record: lineio.RecordRange{Bytes: length, Framing: lineio.RecordFramingOpaque}, BufferBytes: window}
		want := strings.Fields(string(body))
		ordinal, cursor := 0, 0
		for field, err := range lineio.RecordFields(t.Context(), request) {
			if err != nil {
				t.Fatal(err)
			}
			if ordinal >= len(want) {
				t.Fatal("native source invented field")
			}
			offset := bytes.Index(sourceBytes[cursor:], []byte(want[ordinal])) + cursor
			if int(field.Offset) != offset || field.Bytes.Uint64() != uint64(len(want[ordinal])) || field.Framing != lineio.RecordFramingOpaque {
				t.Fatalf("field=%+v,want exact offset%d/length%d", field, offset, len(want[ordinal]))
			}
			cursor = offset + len(want[ordinal])
			ordinal++
		}
		if ordinal != len(want) {
			t.Fatalf("native fields=%d,want%d", ordinal, len(want))
		}
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		refusals := 0
		for field, err := range lineio.RecordFields(ctx, request) {
			refusals++
			if !errors.Is(err, context.Canceled) || field != (lineio.RecordRange{}) {
				t.Fatalf("cancelled field/error=%+v/%v,wantzero/nativecause", field, err)
			}
		}
		if refusals != 1 {
			t.Fatalf("cancellation observations%d,wantone", refusals)
		}
	})
}

func TestRecordFieldsNativeWindowLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, text string }{
		{"neutral ascii whitespace", " "}, {"native ascii field sequence", "go test\n"}, {"neutral unicode whitespace", "\u0085\u2003"}, {"invalid UTF8 and NUL retain exact bytes", "\xff\x00 field"},
		{"field below working window", strings.Repeat("x", 4095) + " end"}, {"field at working window", strings.Repeat("x", 4096) + " end"}, {"field above working window", strings.Repeat("x", 4097) + " end"}, {"field beyond scanner token limit", strings.Repeat("x", 64*1024+1) + " end"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			text := tc.text
			t.Parallel()
			length, err := core.NewByteLength(uint64(len(text)))
			if err != nil {
				t.Fatal(err)
			}
			window, err := core.NewByteCount(4096)
			if err != nil {
				t.Fatal(err)
			}
			request := lineio.RecordFragmentRequest{Source: strings.NewReader(text), Record: lineio.RecordRange{Bytes: length, Framing: lineio.RecordFramingOpaque}, BufferBytes: window}
			fields := strings.Fields(text)
			ordinal := 0
			for got, err := range lineio.RecordFields(t.Context(), request) {
				if err != nil {
					t.Fatal(err)
				}
				if ordinal >= len(fields) {
					t.Fatal("extra field")
				}
				size, _ := got.Bytes.Int64()
				if text[int64(got.Offset):int64(got.Offset)+size] != fields[ordinal] {
					t.Fatal("native field bytes differ")
				}
				ordinal++
			}
			if ordinal != len(fields) {
				t.Fatalf("field count%d,want%d", ordinal, len(fields))
			}
		})
	}
}

func TestRecordFieldsFinalCallbackCancellationStaysNativeRefusal(t *testing.T) {
	t.Parallel()
	length, err := core.NewByteLength(5)
	if err != nil {
		t.Fatal(err)
	}
	window, err := core.NewByteCount(4096)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	fields, refusals := 0, 0
	for got, err := range lineio.RecordFields(ctx, lineio.RecordFragmentRequest{Source: strings.NewReader("field"), Record: lineio.RecordRange{Bytes: length, Framing: lineio.RecordFramingOpaque}, BufferBytes: window}) {
		if err != nil {
			refusals++
			if !errors.Is(err, context.Canceled) || got != (lineio.RecordRange{}) {
				t.Fatalf("refusal=%+v/%v,wantzero/nativecancel", got, err)
			}
			continue
		}
		fields++
		if got.Bytes.Uint64() != 5 {
			t.Fatalf("fieldextent%d,want5", got.Bytes.Uint64())
		}
		cancel()
	}
	if fields != 1 || refusals != 1 {
		t.Fatalf("fields/refusals%d/%d,wantoneprovisional/onenativecause", fields, refusals)
	}
}

type nativeFieldRefusalSource struct {
	*bytes.Reader
	stop  int64
	cause error
}

func (s nativeFieldRefusalSource) ReadAt(p []byte, offset int64) (int, error) {
	if offset >= s.stop {
		return 0, s.cause
	}
	if int64(len(p)) > s.stop-offset {
		n, err := s.Reader.ReadAt(p[:s.stop-offset], offset)
		return n, errors.Join(err, s.cause)
	}
	return s.Reader.ReadAt(p, offset)
}

func TestRecordFieldsLateSourceFailureRefusesCompletion(t *testing.T) {
	t.Parallel()
	body := []byte(strings.Repeat("field ", 32) + "last")
	length, err := core.NewByteLength(uint64(len(body)))
	if err != nil {
		t.Fatal(err)
	}
	window, err := core.NewByteCount(16)
	if err != nil {
		t.Fatal(err)
	}
	fields, refusals := 0, 0
	source := nativeFieldRefusalSource{Reader: bytes.NewReader(body), stop: 80, cause: errors.Join(io.EOF, io.ErrClosedPipe)}
	for got, err := range lineio.RecordFields(t.Context(), lineio.RecordFragmentRequest{Source: source, Record: lineio.RecordRange{Bytes: length, Framing: lineio.RecordFramingOpaque}, BufferBytes: window}) {
		if err != nil {
			refusals++
			if !errors.Is(err, io.ErrClosedPipe) || got != (lineio.RecordRange{}) {
				t.Fatalf("refusal=%+v/%v,wantzero/nativecause", got, err)
			}
			continue
		}
		fields++
	}
	if fields == 0 || fields >= 33 || refusals != 1 {
		t.Fatalf("provisionalfields/refusals%d/%d,wantprefixandonecompletionrefusal", fields, refusals)
	}
}
