package jsonio_test

import (
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"io"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/jsonio"
	"github.com/deliri/primitive/v2026/lineio"
)

func FuzzProjectionRecordMatchesIndependentNativeDocument(f *testing.F) {
	for _, input := range []string{"", "\n", " \t\r\n", "{}\n", "{\"count\":1,\"text\":\"value\"}\n", "{\"count\":\"wrong\"}\n", "{\"count\":\"wrong\"\n", "{\"alien\":1}\n", "{\"count\":1,\"count\":2}\n", "{} {}\n", "\xff\n", "null\n", "{}\n{\"count\":2}\n"} {
		f.Add(input)
	}
	f.Fuzz(func(t *testing.T, input string) {
		source := strings.NewReader(input)
		got, err := jsonio.DecodeProjectionRecord[projectionRecordDocument](t.Context(), jsonio.ProjectionRecordRequest{Reader: projectionRecordReader(t, source), SyntaxSource: source})
		if input == "" {
			if err != io.EOF || got.Validate() == nil {
				t.Fatalf("empty source = (%+v,%v), want zero and bare EOF", got, err)
			}
			return
		}
		first, _, hasLF := strings.Cut(input, "\n")
		if hasLF {
			first += "\n"
		}
		trimmed := strings.Trim(first, core.JSONWhitespace)
		if err != nil || got.Validate() != nil || got.Record.Bytes.Uint64() != uint64(len(first)) || got.ContentBytes.Uint64() != uint64(len(trimmed)) {
			t.Fatalf("physical observation = (%+v,%v), want first exact native frame of %d bytes", got, err, len(first))
		}
		if trimmed == "" {
			if got.Syntax != jsonio.RecordSyntaxBlank || got.DecodeError != nil || got.Value != (projectionRecordDocument{}) {
				t.Fatalf("blank observation = %+v, want no value or refusal", got)
			}
			return
		}
		grammarValid := jsontext.Value(first).IsValid()
		if (got.Syntax == jsonio.RecordSyntaxComplete) != grammarValid || (got.SyntaxError == nil) != grammarValid {
			t.Fatalf("native grammar = (%v,%v), want independent native validity %t", got.Syntax, got.SyntaxError, grammarValid)
		}
		var expected projectionRecordDocument
		typedErr := jsonv2.Unmarshal([]byte(first), &expected, jsonv2.RejectUnknownMembers(true))
		accepted := grammarValid && jsontext.Value(trimmed).Kind() == jsontext.KindBeginObject && typedErr == nil
		if (got.DecodeError == nil) != accepted || !accepted && got.Value != (projectionRecordDocument{}) || accepted && got.Value != expected {
			t.Fatalf("typed observation = (%+v,%v), want independent admission %t and exact %+v or zero", got.Value, got.DecodeError, accepted, expected)
		}
	})
}

func TestProjectionRecordDrainAndOffsetsStayNative(t *testing.T) {
	t.Parallel()
	first := "{\"count\":\"wrong\"}\r\n"
	second := " \t{\"count\":2}\r\n"
	source := strings.NewReader(first + second)
	reader := projectionRecordReader(t, source)
	refused, err := jsonio.DecodeProjectionRecord[projectionRecordDocument](t.Context(), jsonio.ProjectionRecordRequest{Reader: reader, SyntaxSource: source})
	if err != nil || refused.DecodeError == nil || refused.Syntax != jsonio.RecordSyntaxComplete || refused.Record.Bytes.Uint64() != uint64(len(first)) {
		t.Fatalf("first native record = (%+v,%v), want semantic refusal and complete original frame", refused, err)
	}
	accepted, err := jsonio.DecodeProjectionRecord[projectionRecordDocument](t.Context(), jsonio.ProjectionRecordRequest{Reader: reader, SyntaxSource: source, Offset: lineio.SourceByteOffset(len(first))})
	if err != nil || accepted.Validate() != nil || accepted.DecodeError != nil || accepted.Value != (projectionRecordDocument{Count: 2}) || accepted.Record.Offset != lineio.SourceByteOffset(len(first)) || accepted.Record.Bytes.Uint64() != uint64(len(second)) {
		t.Fatalf("second native record = (%+v,%v), want next exact original frame and value", accepted, err)
	}
	last, err := jsonio.DecodeProjectionRecord[projectionRecordDocument](t.Context(), jsonio.ProjectionRecordRequest{Reader: reader, SyntaxSource: source, Offset: lineio.SourceByteOffset(len(first) + len(second))})
	if err != io.EOF || last.Validate() == nil {
		t.Fatalf("completed native stream = (%+v,%v), want no further record", last, err)
	}
}
