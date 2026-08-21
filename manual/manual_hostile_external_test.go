package manual_test

import (
	"bytes"
	json "encoding/json/v2"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/manual"
)

type testTopic uint8

const (
	testTopicUnknown testTopic = iota
	testTopicOpen
	testTopicClose
	testTopicInvalid
)

func (t testTopic) Validate() error {
	if t == testTopicOpen || t == testTopicClose {
		return nil
	}
	return core.ErrPrimitiveContract
}

func (t testTopic) ManualTopic() manual.TopicName {
	switch t {
	case testTopicOpen:
		return "open"
	case testTopicClose:
		return "close"
	case testTopicUnknown, testTopicInvalid:
		return ""
	}
	return ""
}

func TestValueValidationHostileBoundaryMatrix(t *testing.T) {
	t.Parallel()
	cases := []struct {
		wantErr error
		name    string
		value   manual.Line
	}{
		{name: "ordinary sentence is admitted", value: "Explain the result in plain language."},
		{name: "single visible rune is admitted", value: "x"},
		{name: "unicode customer text is admitted", value: "Résumé ready."},
		{name: "punctuation is admitted", value: "Success: nothing else changed."},
		{name: "exact byte ceiling is admitted", value: manual.Line(strings.Repeat("a", manual.MaximumLineBytes))},
		{name: "empty is refused", wantErr: core.ErrManualContract},
		{name: "leading space is refused", value: " leading", wantErr: core.ErrManualContract},
		{name: "trailing space is refused", value: "trailing ", wantErr: core.ErrManualContract},
		{name: "line feed is refused", value: "one\ntwo", wantErr: core.ErrManualContract},
		{name: "carriage return is refused", value: "one\rtwo", wantErr: core.ErrManualContract},
		{name: "tab is refused", value: "one\ttwo", wantErr: core.ErrManualContract},
		{name: "nul is refused", value: "one\x00two", wantErr: core.ErrManualContract},
		{name: "invalid utf8 is refused", value: manual.Line(string([]byte{0xff})), wantErr: core.ErrManualContract},
		{name: "one above byte ceiling is refused", value: manual.Line(strings.Repeat("a", manual.MaximumLineBytes+1)), wantErr: core.ErrManualContract},
		{name: "only space is refused", value: " ", wantErr: core.ErrManualContract},
		{name: "only newline is refused", value: "\n", wantErr: core.ErrManualContract},
		{name: "leading nonbreaking space is refused", value: "\u00a0text", wantErr: core.ErrManualContract},
		{name: "trailing nonbreaking space is refused", value: "text\u00a0", wantErr: core.ErrManualContract},
		{name: "escape is refused", value: "one\x1btwo", wantErr: core.ErrManualContract},
		{name: "delete control is refused", value: "one\x7ftwo", wantErr: core.ErrManualContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotErr := tc.value.Validate()
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("Line.Validate() error = %v, want %v", gotErr, tc.wantErr)
			}
		})
	}
}

func TestScalarConstructorsLayerTriad(t *testing.T) {
	t.Parallel()
	wantLine := "Readable customer guidance."
	line, err := manual.ParseLine(wantLine)
	if err != nil || line.String() != wantLine {
		t.Fatalf("ParseLine(valid) = (%q, %v), want (%q, nil)", line, err, wantLine)
	}
	if _, gotErr := manual.ParseLine(""); !errors.Is(gotErr, core.ErrManualContract) {
		t.Fatalf("ParseLine(invalid) error = %v, want %v", gotErr, core.ErrManualContract)
	}
	wantTopic := "close-proof"
	topic, err := manual.NewTopicName(wantTopic)
	if err != nil || topic.String() != wantTopic {
		t.Fatalf("NewTopicName(valid) = (%q, %v), want (%q, nil)", topic, err, wantTopic)
	}
	if _, gotErr := manual.NewTopicName("close--proof"); !errors.Is(gotErr, core.ErrManualContract) {
		t.Fatalf("NewTopicName(invalid) error = %v, want %v", gotErr, core.ErrManualContract)
	}
	schema, err := manual.NewSchema(manual.SchemaV1.String())
	if err != nil || schema != manual.SchemaV1 {
		t.Fatalf("NewSchema(valid) = (%q, %v), want (%q, nil)", schema, err, manual.SchemaV1)
	}
	if _, gotErr := manual.NewSchema(""); !errors.Is(gotErr, core.ErrManualContract) {
		t.Fatalf("NewSchema(invalid) error = %v, want %v", gotErr, core.ErrManualContract)
	}
}

func TestBookValidationLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		wantErr error
		mutate  func(*manual.Book[testTopic])
		name    string
	}{
		{name: "positive complete book is admitted"},
		{name: "negative duplicate topic is refused", mutate: func(book *manual.Book[testTopic]) { book.Pages[1].Topic = testTopicOpen }, wantErr: core.ErrManualContract},
		{name: "neutral optional definitions may be absent", mutate: func(book *manual.Book[testTopic]) { book.Pages[0].Definitions = nil }},
		{name: "missing success is refused", mutate: func(book *manual.Book[testTopic]) { book.Pages[0].Outcome.Success = nil }, wantErr: core.ErrManualContract},
		{name: "foreign related topic is refused", mutate: func(book *manual.Book[testTopic]) { book.Pages[0].Related = []testTopic{testTopicInvalid} }, wantErr: core.ErrManualContract},
		{name: "self relation is refused", mutate: func(book *manual.Book[testTopic]) { book.Pages[0].Related = []testTopic{testTopicOpen} }, wantErr: core.ErrManualContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			book := validBook(t)
			if tc.mutate != nil {
				tc.mutate(&book)
			}
			gotErr := book.Validate()
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("Book.Validate() error = %v, want %v", gotErr, tc.wantErr)
			}
		})
	}
}

func TestHumanProjectionLayerTriad(t *testing.T) {
	t.Parallel()
	book := validBook(t)
	request := manual.RenderRequest[testTopic]{Book: book, View: manual.ViewManual, Selection: manual.Selection[testTopic]{Mode: manual.SelectionModeTopic, Topic: testTopicOpen}}
	var first bytes.Buffer
	if err := manual.WriteText(&first, request); err != nil {
		t.Fatalf("WriteText(valid) error = %v, want nil", err)
	}
	var second bytes.Buffer
	if err := manual.WriteText(&second, request); err != nil {
		t.Fatalf("WriteText(repeat) error = %v, want nil", err)
	}
	if got, want := second.String(), first.String(); got != want {
		t.Fatalf("WriteText(repeat) = %q, want %q", got, want)
	}
	if !strings.Contains(first.String(), string(book.Pages[0].Summary)) {
		t.Fatalf("WriteText(valid) = %q, want product summary %q", first.String(), book.Pages[0].Summary)
	}

	invalid := request
	invalid.Selection.Topic = testTopicClose
	invalid.Book.Pages = invalid.Book.Pages[:1]
	var rejected bytes.Buffer
	if gotErr := manual.WriteText(&rejected, invalid); !errors.Is(gotErr, core.ErrManualContract) {
		t.Fatalf("WriteText(invalid) error = %v, want %v", gotErr, core.ErrManualContract)
	}
	if got, want := rejected.Len(), 0; got != want {
		t.Fatalf("WriteText(invalid) bytes = %d, want %d", got, want)
	}

	neutral := request
	neutral.Selection = manual.Selection[testTopic]{Mode: manual.SelectionModeIndex}
	var index bytes.Buffer
	if err := manual.WriteText(&index, neutral); err != nil {
		t.Fatalf("WriteText(index) error = %v, want nil", err)
	}
	if !strings.Contains(index.String(), string(book.Title)) {
		t.Fatalf("WriteText(index) = %q, want title %q", index.String(), book.Title)
	}
}

type failingWriter struct{ failure error }

func (w failingWriter) Write([]byte) (int, error) { return 0, w.failure }

type shortWriter struct{}

func (shortWriter) Write(value []byte) (int, error) { return len(value) - 1, nil }

func TestOutputFailurePreservesNativeIdentity(t *testing.T) {
	t.Parallel()
	request := manual.RenderRequest[testTopic]{Book: validBook(t), View: manual.ViewHelp, Selection: manual.Selection[testTopic]{Mode: manual.SelectionModeIndex}}
	native := errors.New("native writer refusal")
	if gotErr := manual.WriteText(failingWriter{failure: native}, request); !errors.Is(gotErr, native) || !errors.Is(gotErr, core.ErrManualWrite) {
		t.Fatalf("WriteText(native failure) error = %v, want %v and %v", gotErr, native, core.ErrManualWrite)
	}
	if gotErr := manual.WriteText(shortWriter{}, request); !errors.Is(gotErr, io.ErrShortWrite) || !errors.Is(gotErr, core.ErrManualWrite) {
		t.Fatalf("WriteText(short write) error = %v, want %v and %v", gotErr, io.ErrShortWrite, core.ErrManualWrite)
	}
	report, err := manual.Project(validBook(t))
	if err != nil {
		t.Fatalf("Project(valid) error = %v, want nil", err)
	}
	if gotErr := manual.WriteJSON(shortWriter{}, report); !errors.Is(gotErr, io.ErrShortWrite) || !errors.Is(gotErr, core.ErrManualWrite) {
		t.Fatalf("WriteJSON(short write) error = %v, want %v and %v", gotErr, io.ErrShortWrite, core.ErrManualWrite)
	}
}

func TestMachineProjectionLayerTriad(t *testing.T) {
	t.Parallel()
	book := validBook(t)
	book.Offering = manualOfferingFixture(t, "kernel-manual")
	report, err := manual.Project(book)
	if err != nil {
		t.Fatalf("Project(valid) error = %v, want nil", err)
	}
	var encoded bytes.Buffer
	if err := manual.WriteJSON(&encoded, report); err != nil {
		t.Fatalf("WriteJSON(valid) error = %v, want nil", err)
	}
	var decoded manual.Report
	if err := json.Unmarshal(encoded.Bytes(), &decoded); err != nil {
		t.Fatalf("json.Unmarshal(WriteJSON) error = %v, want nil", err)
	}
	if err := decoded.Validate(); err != nil {
		t.Fatalf("Report.Validate(round trip) error = %v, want nil", err)
	}
	if got, want := decoded.Schema, manual.SchemaV1; got != want {
		t.Fatalf("Report.Schema = %q, want %q", got, want)
	}
	if got, want := decoded.Offering, book.Offering; got != want {
		t.Fatalf("Report.Offering = %q, want %q", got, want)
	}

	report.Schema = ""
	var rejected bytes.Buffer
	if gotErr := manual.WriteJSON(&rejected, report); !errors.Is(gotErr, core.ErrManualContract) {
		t.Fatalf("WriteJSON(invalid) error = %v, want %v", gotErr, core.ErrManualContract)
	}
	if got, want := rejected.Len(), 0; got != want {
		t.Fatalf("WriteJSON(invalid) bytes = %d, want %d", got, want)
	}

	book.Pages[0].Summary = "caller mutation"
	if got, want := decoded.Pages[0].Summary, manual.Line("Open one bug record."); got != want {
		t.Fatalf("projected summary after source mutation = %q, want %q", got, want)
	}
}

func validBook(t testing.TB) manual.Book[testTopic] {
	t.Helper()
	return manual.Book[testTopic]{Offering: manualOfferingFixture(t, "manual-fixture"), Title: "Bug command guide", Summary: "Use this guide to understand each command before running it.", Pages: []manual.Page[testTopic]{
		{Topic: testTopicOpen, Summary: "Open one bug record.", Usage: []manual.Line{"bug open login_auth"}, Prerequisites: []manual.Line{"Run this inside a Git repository."}, Changes: []manual.Line{"Creates one local bug record."}, Unchanged: []manual.Line{"Does not upload source code."}, Definitions: []manual.Definition{{Term: "bug record", Meaning: "A local file containing typed defect facts."}}, Examples: []manual.Line{"bug open login_auth"}, Outcome: manual.Outcome{Success: []manual.Line{"Prints the created record name."}, Refusal: []manual.Line{"Prints why no record was created."}}, Related: []testTopic{testTopicClose}},
		{Topic: testTopicClose, Summary: "Close one proven bug record.", Usage: []manual.Line{"bug close login_auth"}, Changes: []manual.Line{"Records verified closure evidence."}, Unchanged: []manual.Line{"Does not rewrite source files."}, Examples: []manual.Line{"bug close login_auth"}, Outcome: manual.Outcome{Success: []manual.Line{"Prints the closure receipt."}, Refusal: []manual.Line{"Keeps the record open and explains why."}}, Related: []testTopic{testTopicOpen}},
	}}
}

func manualOfferingFixture(t testing.TB, token string) core.Offering {
	t.Helper()
	offering := core.Offering{Token: token}
	if err := offering.Validate(); err != nil {
		t.Fatalf("Offering.Validate() error = %v, want nil", err)
	}
	return offering
}
