package jsonio_test

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/jsonio"
	"github.com/deliri/primitive/v2026/lineio"
)

type projectionRecordDocument struct {
	Text  string `json:"text"`
	Count int64  `json:"count"`
}

func projectionRecordReader(t testing.TB, source io.Reader) *lineio.Reader {
	t.Helper()
	buffer, err := core.NewByteCount(65536)
	if err != nil {
		t.Fatal(err)
	}
	reader, err := lineio.New(lineio.Request{Source: source, BufferBytes: buffer})
	if err != nil {
		t.Fatal(err)
	}
	return reader
}

func TestProjectionRecordSeparatesFramingGrammarAndMeaning(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		input   string
		want    projectionRecordDocument
		syntax  jsonio.RecordSyntax
		refused bool
	}{
		{name: "blank LF", input: "\n", syntax: jsonio.RecordSyntaxBlank},
		{name: "blank CRLF", input: " \t\r\n", syntax: jsonio.RecordSyntaxBlank},
		{name: "blank EOF", input: " \t\r", syntax: jsonio.RecordSyntaxBlank},
		{name: "empty object", input: "{}\n", syntax: jsonio.RecordSyntaxComplete},
		{name: "declared fields LF", input: "{\"count\":1,\"text\":\"value\"}\n", want: projectionRecordDocument{Count: 1, Text: "value"}, syntax: jsonio.RecordSyntaxComplete},
		{name: "declared fields EOF", input: "{\"count\":1,\"text\":\"value\"}", want: projectionRecordDocument{Count: 1, Text: "value"}, syntax: jsonio.RecordSyntaxComplete},
		{name: "trimmed physical extent", input: " \t{\"count\":1}\t\r\n", want: projectionRecordDocument{Count: 1}, syntax: jsonio.RecordSyntaxComplete},
		{name: "escaped newline remains data", input: "{\"text\":\"first\\nsecond\"}\n", want: projectionRecordDocument{Text: "first\nsecond"}, syntax: jsonio.RecordSyntaxComplete},
		{name: "unknown field complete", input: "{\"alien\":1}\n", syntax: jsonio.RecordSyntaxComplete, refused: true},
		{name: "unknown field before tear", input: "{\"alien\":1\n", syntax: jsonio.RecordSyntaxInvalid, refused: true},
		{name: "wrong scalar complete", input: "{\"count\":\"wrong\"}\n", syntax: jsonio.RecordSyntaxComplete, refused: true},
		{name: "wrong scalar before tear", input: "{\"count\":\"wrong\"\n", syntax: jsonio.RecordSyntaxInvalid, refused: true},
		{name: "float for integer", input: "{\"count\":1.5}\n", syntax: jsonio.RecordSyntaxComplete, refused: true},
		{name: "integer overflow", input: "{\"count\":9223372036854775808}\n", syntax: jsonio.RecordSyntaxComplete, refused: true},
		{name: "wrong text scalar", input: "{\"text\":1}\n", syntax: jsonio.RecordSyntaxComplete, refused: true},
		{name: "array root", input: "[]\n", syntax: jsonio.RecordSyntaxComplete, refused: true},
		{name: "string root", input: "\"text\"\n", syntax: jsonio.RecordSyntaxComplete, refused: true},
		{name: "number root", input: "1\n", syntax: jsonio.RecordSyntaxComplete, refused: true},
		{name: "null root refused by policy", input: "null\n", syntax: jsonio.RecordSyntaxComplete, refused: true},
		{name: "duplicate field", input: "{\"count\":1,\"count\":2}\n", syntax: jsonio.RecordSyntaxInvalid, refused: true},
		{name: "second root value", input: "{} {}\n", syntax: jsonio.RecordSyntaxInvalid, refused: true},
		{name: "trailing invalid token", input: "{}x\n", syntax: jsonio.RecordSyntaxInvalid, refused: true},
		{name: "truncated object", input: "{\n", syntax: jsonio.RecordSyntaxInvalid, refused: true},
		{name: "truncated scalar", input: "{\"text\":\"cut\n", syntax: jsonio.RecordSyntaxInvalid, refused: true},
		{name: "BOM is not JSON whitespace", input: "\ufeff{}\n", syntax: jsonio.RecordSyntaxInvalid, refused: true},
		{name: "Unicode space is not JSON whitespace", input: "\u00a0{}\n", syntax: jsonio.RecordSyntaxInvalid, refused: true},
		{name: "vertical tab is not JSON whitespace", input: "\v{}\n", syntax: jsonio.RecordSyntaxInvalid, refused: true},
		{name: "NUL is not a JSON token", input: "\x00{}\n", syntax: jsonio.RecordSyntaxInvalid, refused: true},
		{name: "malformed UTF8", input: "{\"text\":\"\xff\"}\n", syntax: jsonio.RecordSyntaxInvalid, refused: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			source := strings.NewReader(tc.input)
			got, err := jsonio.DecodeProjectionRecord[projectionRecordDocument](t.Context(), jsonio.ProjectionRecordRequest{Reader: projectionRecordReader(t, source), SyntaxSource: source})
			wantFraming := lineio.RecordFramingEOF
			if strings.HasSuffix(tc.input, "\n") {
				wantFraming = lineio.RecordFramingLF
			}
			if err != nil || got.Validate() != nil || got.Syntax != tc.syntax || got.Record.Offset != 0 || got.Record.Bytes.Uint64() != uint64(len(tc.input)) || got.Record.Framing != wantFraming || got.ContentBytes.Uint64() != uint64(len(strings.Trim(tc.input, core.JSONWhitespace))) || (got.DecodeError != nil) != tc.refused {
				t.Fatalf("record = (%+v,%v), want exact framing/extent, syntax %v and refusal %t", got, err, tc.syntax, tc.refused)
			}
			if tc.refused && got.Value != (projectionRecordDocument{}) || !tc.refused && got.Value != tc.want {
				t.Fatalf("projection = %+v, want zero on refusal or exact %+v", got.Value, tc.want)
			}
			if (got.SyntaxError != nil) != (tc.syntax == jsonio.RecordSyntaxInvalid) {
				t.Fatalf("syntax cause = %v, want refusal only for invalid grammar", got.SyntaxError)
			}
		})
	}
}

func TestProjectionRecordWithdrawsFailedOrChangedObservation(t *testing.T) {
	t.Parallel()
	for _, boundary := range []string{"late native read failure", "closed positional source", "different positional bytes", "different positional extent", "canceled source", "clean empty EOF", "uninspected semantic refusal"} {
		t.Run(boundary, func(t *testing.T) {
			t.Parallel()
			ctx := t.Context()
			input := "{\"count\":\"wrong\"}\n"
			var source io.Reader = strings.NewReader(input)
			var positional lineio.RecordSource = strings.NewReader(input)
			var want error = core.ErrJSONContract
			switch boundary {
			case "late native read failure", "closed positional source":
				file, err := os.Create(filepath.Join(t.TempDir(), "closed"))
				if err != nil {
					t.Fatal(err)
				}
				if err := file.Close(); err != nil {
					t.Fatal(err)
				}
				want = fs.ErrClosed
				if boundary == "late native read failure" {
					source = io.MultiReader(strings.NewReader("{\"count\":1} "), file)
					positional = nil
				} else {
					positional = file
				}
			case "different positional bytes":
				positional = strings.NewReader("{\"count\":\"other\"}\n")
			case "different positional extent":
				positional = strings.NewReader("{")
			case "canceled source":
				canceled, cancel := context.WithCancelCause(ctx)
				cancel(fs.ErrPermission)
				ctx = canceled
				want = context.Canceled
			case "clean empty EOF":
				source = strings.NewReader("")
				positional = nil
				want = io.EOF
			case "uninspected semantic refusal":
				positional = nil
			}
			got, err := jsonio.DecodeProjectionRecord[projectionRecordDocument](ctx, jsonio.ProjectionRecordRequest{Reader: projectionRecordReader(t, source), SyntaxSource: positional})
			if boundary == "uninspected semantic refusal" {
				if err != nil || got.Validate() != nil || got.Syntax != jsonio.RecordSyntaxUninspected || got.DecodeError == nil || got.Value != (projectionRecordDocument{}) {
					t.Fatalf("uninspected = (%+v,%v), want no grammar claim and zero rejected projection", got, err)
				}
				return
			}
			if !errors.Is(err, want) || got.Validate() == nil || got.Value != (projectionRecordDocument{}) {
				t.Fatalf("withdrawn observation = (%+v,%v), want zero and %v", got, err, want)
			}
			if boundary == "canceled source" && !errors.Is(err, fs.ErrPermission) {
				t.Fatalf("cancellation lost cause: %v", err)
			}
		})
	}
}
