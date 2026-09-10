package manual_test

import (
	"bytes"
	json "encoding/json/v2"
	"errors"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/manual"
)

func FuzzLineGrammar(f *testing.F) {
	forbidden := regexp.MustCompile(`[\p{Cc}\p{Zl}\p{Zp}]`)
	for _, text := range []string{"A line.", "Résumé.", "👩‍💻", strings.Repeat("x", manualFormerLineBytes+1)} {
		line, err := manual.ParseLine(text)
		if err != nil {
			f.Fatalf("seed=%v, want valid line", err)
		}
		f.Add(line.String())
	}
	for _, text := range []string{"", " x", "x ", "x\ny", "x\u2028y", string([]byte{0xff})} {
		f.Add(text)
	}
	f.Fuzz(func(t *testing.T, text string) {
		want := text != "" && utf8.ValidString(text) && strings.TrimSpace(text) == text && !forbidden.MatchString(text)
		got, err := manual.ParseLine(text)
		if (err == nil) != want || (want && (got.String() != text || got.Validate() != nil)) || (!want && (got != "" || !errors.Is(err, core.ErrManualContract))) {
			t.Fatalf("line=%q error=%v, want admitted=%t and exact text or zero", got, err, want)
		}
	})
}

func FuzzSchemaTextAndJSON(f *testing.F) {
	schema, err := manual.NewSchema(manual.SchemaV1.String())
	if err != nil {
		f.Fatal(err)
	}
	canonical, err := schema.MarshalJSON()
	if err != nil {
		f.Fatal(err)
	}
	f.Add(canonical)
	f.Add([]byte(schema.String()))
	f.Add([]byte("null"))
	f.Add([]byte(`"future"`))
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, data []byte) {
		got, err := manual.NewSchema(string(data))
		want := string(data) == manual.SchemaV1Token
		if (err == nil) != want || (want && got != manual.SchemaV1) || (!want && (got != manual.SchemaUnknown || !errors.Is(err, core.ErrManualContract))) {
			t.Fatalf("schema text=%v/%v, want admitted=%t", got, err, want)
		}
		var native string
		nativeErr := json.Unmarshal(data, &native)
		wantJSON := nativeErr == nil && native == manual.SchemaV1Token
		decoded := manual.SchemaV1
		decodeErr := decoded.UnmarshalJSON(data)
		if (decodeErr == nil) != wantJSON || decoded != manual.SchemaV1 || (!wantJSON && (!errors.Is(decodeErr, core.ErrJSONContract) || !errors.Is(decodeErr, core.ErrManualContract))) {
			t.Fatalf("schema JSON=%v/%v, want admitted=%t and retained published schema", decoded, decodeErr, wantJSON)
		}
		if wantJSON {
			encoded, err := decoded.MarshalJSON()
			if err != nil || !bytes.Equal(encoded, canonical) {
				t.Fatalf("schema output=%q/%v, want %q", encoded, err, canonical)
			}
		}
	})
}

type bookTextField uint8

const (
	bookTextTitle bookTextField = iota
	bookTextSummary
	bookTextPageSummary
	bookTextUsage
	bookTextFieldLimit
)

func FuzzBookProjectionAndOutput(f *testing.F) {
	forbidden := regexp.MustCompile(`[\p{Cc}\p{Zl}\p{Zp}]`)
	fixture := namedBook(f, 1)
	for field, line := range []manual.Line{fixture.Title, fixture.Summary, fixture.Pages[0].Summary, fixture.Pages[0].Usage[0]} {
		if err := line.Validate(); err != nil {
			f.Fatal(err)
		}
		f.Add(line.String(), uint8(field))
	}
	f.Add("", uint8(bookTextSummary))
	f.Add("bad\u2029line", uint8(bookTextPageSummary))
	f.Fuzz(func(t *testing.T, text string, field uint8) {
		book := namedBook(t, 1)
		want := text != "" && utf8.ValidString(text) && strings.TrimSpace(text) == text && !forbidden.MatchString(text)
		switch bookTextField(field % uint8(bookTextFieldLimit)) {
		case bookTextTitle:
			book.Title = manual.Line(text)
		case bookTextSummary:
			book.Summary = manual.Line(text)
		case bookTextPageSummary:
			book.Pages[0].Summary = manual.Line(text)
		case bookTextUsage:
			book.Pages[0].Usage[0] = manual.Line(text)
		}
		report, err := manual.Project(book)
		if (err == nil) != want {
			t.Fatalf("projection=%v, want admitted=%t", err, want)
		}
		if !want {
			if !errors.Is(err, core.ErrManualContract) || len(report.Pages) != 0 {
				t.Fatalf("refused projection=%v/%v, want zero typed refusal", report, err)
			}
			var sink manualBenchmarkSink
			writeErr := manual.WriteText(&sink, manual.RenderRequest[namedTopic]{Book: book, View: manual.ViewManual, Selection: manual.Selection[namedTopic]{Mode: manual.SelectionModeTopic, Topic: book.Pages[0].Topic}})
			if !errors.Is(writeErr, core.ErrManualContract) || sink.bytes != 0 {
				t.Fatalf("refused write=%d/%v, want zero and typed refusal", sink.bytes, writeErr)
			}
			return
		}
		if report.Validate() != nil || report.Title != book.Title || report.Summary != book.Summary || len(report.Pages) != 1 || report.Pages[0].Summary != book.Pages[0].Summary || len(report.Pages[0].Usage) != 1 || report.Pages[0].Usage[0] != book.Pages[0].Usage[0] {
			t.Fatalf("projected report=%v, want exact source fields", report)
		}
		var sink manualBenchmarkSink
		if err := manual.WriteJSON(&sink, report); err != nil || sink.bytes == 0 {
			t.Fatalf("JSON bytes=%d error=%v, want nonempty valid report", sink.bytes, err)
		}
	})
}
