package manual_test

import (
	"errors"
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
		{name: "former byte ceiling is admitted", value: manual.Line(strings.Repeat("a", manualFormerLineBytes))},
		{name: "empty is refused", wantErr: core.ErrManualContract},
		{name: "leading space is refused", value: " leading", wantErr: core.ErrManualContract},
		{name: "trailing space is refused", value: "trailing ", wantErr: core.ErrManualContract},
		{name: "line feed is refused", value: "one\ntwo", wantErr: core.ErrManualContract},
		{name: "carriage return is refused", value: "one\rtwo", wantErr: core.ErrManualContract},
		{name: "tab is refused", value: "one\ttwo", wantErr: core.ErrManualContract},
		{name: "nul is refused", value: "one\x00two", wantErr: core.ErrManualContract},
		{name: "invalid utf8 is refused", value: manual.Line(string([]byte{0xff})), wantErr: core.ErrManualContract},
		{name: "one above former byte ceiling is admitted", value: manual.Line(strings.Repeat("a", manualFormerLineBytes+1))},
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
			got, parseErr := manual.ParseLine(tc.value.String())
			if !errors.Is(parseErr, tc.wantErr) || (tc.wantErr == nil && got != tc.value) || (tc.wantErr != nil && got != "") {
				t.Fatalf("ParseLine=%q/%v, want exact input or zero with %v", got, parseErr, tc.wantErr)
			}
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("Line.Validate() error = %v, want %v", gotErr, tc.wantErr)
			}
		})
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

func validBook(t testing.TB) manual.Book[testTopic] {
	t.Helper()
	return manual.Book[testTopic]{Offering: manualOfferingFixture(t, "manual-fixture"), Title: "Issue command guide", Summary: "Use this guide to understand each command before running it.", Pages: []manual.Page[testTopic]{
		{Topic: testTopicOpen, Summary: "Open one issue record.", Usage: []manual.Line{"issue open login_auth"}, Prerequisites: []manual.Line{"Run this inside a Git repository."}, Changes: []manual.Line{"Creates one local issue record."}, Unchanged: []manual.Line{"Does not upload source code."}, Definitions: []manual.Definition{{Term: "issue record", Meaning: "A local file containing typed defect facts."}}, Examples: []manual.Line{"issue open login_auth"}, Outcome: manual.Outcome{Success: []manual.Line{"Prints the created record name."}, Refusal: []manual.Line{"Prints why no record was created."}}, Related: []testTopic{testTopicClose}},
		{Topic: testTopicClose, Summary: "Close one proven issue record.", Usage: []manual.Line{"issue close login_auth"}, Changes: []manual.Line{"Records verified closure evidence."}, Unchanged: []manual.Line{"Does not rewrite source files."}, Examples: []manual.Line{"issue close login_auth"}, Outcome: manual.Outcome{Success: []manual.Line{"Prints the closure receipt."}, Refusal: []manual.Line{"Keeps the record open and explains why."}}, Related: []testTopic{testTopicOpen}},
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
