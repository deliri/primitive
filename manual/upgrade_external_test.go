package manual_test

import (
	"bytes"
	json "encoding/json/v2"
	"errors"
	"io"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/manual"
)

const (
	manualFormerPages        = 64
	manualFormerSectionItems = 32
	manualFormerLineBytes    = 1024
	manualFormerTopicBytes   = 64
	manualStreamWindow       = 4096
)

type namedTopic struct {
	name     manual.TopicName
	identity uint64
}

func (topic namedTopic) Validate() error {
	if topic.identity == 0 {
		return core.ErrPrimitiveContract
	}
	return nil
}
func (topic namedTopic) ManualTopic() manual.TopicName { return topic.name }

func namedBook(t testing.TB, count int) manual.Book[namedTopic] {
	t.Helper()
	book := manual.Book[namedTopic]{Title: "Guide", Summary: "Mechanical command guidance.", Offering: manualOfferingFixture(t, "manual-fixture")}
	for i := range count {
		topic, err := manual.NewTopicName("topic-" + strconv.Itoa(i+1))
		if err != nil {
			t.Fatal(err)
		}
		book.Pages = append(book.Pages, manual.Page[namedTopic]{
			Topic: namedTopic{name: topic, identity: uint64(i + 1)}, Summary: "Command summary.",
			Usage: []manual.Line{"tool command"}, Changes: []manual.Line{"Writes a result."},
			Unchanged: []manual.Line{"Leaves other files alone."}, Examples: []manual.Line{"tool command"},
			Outcome: manual.Outcome{Success: []manual.Line{"Returns the result."}, Refusal: []manual.Line{"Returns a typed refusal."}},
		})
	}
	return book
}
func manualLines(count int) []manual.Line {
	lines := make([]manual.Line, count)
	for i := range lines {
		lines[i] = manual.Line("line-" + strconv.Itoa(i))
	}
	return lines
}

func TestManualContractLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		mutate  func(*manual.Book[namedTopic])
		pages   int
		wantErr error
	}{
		{name: "positive_complete_book", pages: 2, mutate: func(b *manual.Book[namedTopic]) {
			b.Pages[0].Prerequisites = []manual.Line{"A valid prerequisite."}
			b.Pages[0].Definitions = []manual.Definition{{Term: "term", Meaning: "A precise definition."}}
			b.Pages[0].Related = []namedTopic{b.Pages[1].Topic}
		}},
		{name: "positive_large_page_count", pages: manualFormerPages + 1},
		{name: "positive_large_usage_count", pages: 1, mutate: func(b *manual.Book[namedTopic]) { b.Pages[0].Usage = manualLines(manualFormerSectionItems + 1) }},
		{name: "positive_large_definition_count", pages: 1, mutate: func(b *manual.Book[namedTopic]) {
			for _, term := range manualLines(manualFormerSectionItems + 1) {
				b.Pages[0].Definitions = append(b.Pages[0].Definitions, manual.Definition{Term: term, Meaning: "A distinct term."})
			}
		}},
		{name: "positive_large_relation_count", pages: manualFormerSectionItems + 2, mutate: func(b *manual.Book[namedTopic]) {
			for _, p := range b.Pages[1:] {
				b.Pages[0].Related = append(b.Pages[0].Related, p.Topic)
			}
		}},
		{name: "positive_many_window_line", pages: 1, mutate: func(b *manual.Book[namedTopic]) {
			b.Pages[0].Summary = manual.Line(strings.Repeat("x", manualStreamWindow*16+1))
		}},
		{name: "positive_exact_typed_relation", pages: 2, mutate: func(b *manual.Book[namedTopic]) { b.Pages[0].Related = []namedTopic{b.Pages[1].Topic} }},
		{name: "neutral_optional_sections_absent", pages: 1},
		{name: "negative_no_pages", wantErr: core.ErrManualContract},
		{name: "negative_missing_offering", pages: 1, mutate: func(b *manual.Book[namedTopic]) { b.Offering = core.Offering{} }, wantErr: core.ErrManualContract},
		{name: "negative_missing_title", pages: 1, mutate: func(b *manual.Book[namedTopic]) { b.Title = "" }, wantErr: core.ErrManualContract},
		{name: "negative_missing_book_summary", pages: 1, mutate: func(b *manual.Book[namedTopic]) { b.Summary = "" }, wantErr: core.ErrManualContract},
		{name: "negative_invalid_product_identity", pages: 1, mutate: func(b *manual.Book[namedTopic]) { b.Pages[0].Topic.identity = 0 }, wantErr: core.ErrManualContract},
		{name: "negative_invalid_canonical_topic", pages: 1, mutate: func(b *manual.Book[namedTopic]) { b.Pages[0].Topic.name = "Bad" }, wantErr: core.ErrManualContract},
		{name: "negative_duplicate_page_name", pages: 2, mutate: func(b *manual.Book[namedTopic]) { b.Pages[1].Topic.name = b.Pages[0].Topic.name }, wantErr: core.ErrManualContract},
		{name: "negative_missing_page_summary", pages: 1, mutate: func(b *manual.Book[namedTopic]) { b.Pages[0].Summary = "" }, wantErr: core.ErrManualContract},
		{name: "negative_missing_usage", pages: 1, mutate: func(b *manual.Book[namedTopic]) { b.Pages[0].Usage = nil }, wantErr: core.ErrManualContract},
		{name: "negative_missing_changes", pages: 1, mutate: func(b *manual.Book[namedTopic]) { b.Pages[0].Changes = nil }, wantErr: core.ErrManualContract},
		{name: "negative_missing_unchanged", pages: 1, mutate: func(b *manual.Book[namedTopic]) { b.Pages[0].Unchanged = nil }, wantErr: core.ErrManualContract},
		{name: "negative_missing_examples", pages: 1, mutate: func(b *manual.Book[namedTopic]) { b.Pages[0].Examples = nil }, wantErr: core.ErrManualContract},
		{name: "negative_missing_success", pages: 1, mutate: func(b *manual.Book[namedTopic]) { b.Pages[0].Outcome.Success = nil }, wantErr: core.ErrManualContract},
		{name: "negative_missing_refusal", pages: 1, mutate: func(b *manual.Book[namedTopic]) { b.Pages[0].Outcome.Refusal = nil }, wantErr: core.ErrManualContract},
		{name: "negative_invalid_optional_line", pages: 1, mutate: func(b *manual.Book[namedTopic]) { b.Pages[0].Prerequisites = []manual.Line{"\x00"} }, wantErr: core.ErrManualContract},
		{name: "negative_duplicate_line", pages: 1, mutate: func(b *manual.Book[namedTopic]) { b.Pages[0].Usage = append(b.Pages[0].Usage, b.Pages[0].Usage[0]) }, wantErr: core.ErrManualContract},
		{name: "negative_missing_definition_term", pages: 1, mutate: func(b *manual.Book[namedTopic]) { b.Pages[0].Definitions = []manual.Definition{{Meaning: "Defined."}} }, wantErr: core.ErrManualContract},
		{name: "negative_missing_definition_meaning", pages: 1, mutate: func(b *manual.Book[namedTopic]) { b.Pages[0].Definitions = []manual.Definition{{Term: "term"}} }, wantErr: core.ErrManualContract},
		{name: "negative_conflicting_definition", pages: 1, mutate: func(b *manual.Book[namedTopic]) {
			b.Pages[0].Definitions = []manual.Definition{{Term: "term", Meaning: "One."}, {Term: "term", Meaning: "Two."}}
		}, wantErr: core.ErrManualContract},
		{name: "negative_unknown_related_topic", pages: 1, mutate: func(b *manual.Book[namedTopic]) { b.Pages[0].Related = []namedTopic{{identity: 2, name: "absent"}} }, wantErr: core.ErrManualContract},
		{name: "negative_related_identity_borrows_documented_name", pages: 2, mutate: func(b *manual.Book[namedTopic]) {
			alias := b.Pages[1].Topic
			alias.identity++
			b.Pages[0].Related = []namedTopic{alias}
		}, wantErr: core.ErrManualContract},
		{name: "negative_self_relation_through_alias", pages: 1, mutate: func(b *manual.Book[namedTopic]) {
			alias := b.Pages[0].Topic
			alias.identity++
			b.Pages[0].Related = []namedTopic{alias}
		}, wantErr: core.ErrManualContract},
		{name: "negative_duplicate_related_canonical_names", pages: 2, mutate: func(b *manual.Book[namedTopic]) {
			alias := b.Pages[1].Topic
			alias.identity++
			b.Pages[0].Related = []namedTopic{b.Pages[1].Topic, alias}
		}, wantErr: core.ErrManualContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			book := namedBook(t, tc.pages)
			if tc.mutate != nil {
				tc.mutate(&book)
			}
			validation := book.Validate()
			report, projectionErr := manual.Project(book)
			var text bytes.Buffer
			writeErr := manual.WriteText(&text, manual.RenderRequest[namedTopic]{Book: book, View: manual.ViewHelp, Selection: manual.Selection[namedTopic]{Mode: manual.SelectionModeIndex}})
			if !errors.Is(validation, tc.wantErr) || !errors.Is(projectionErr, tc.wantErr) || !errors.Is(writeErr, tc.wantErr) {
				t.Fatalf("validate=%v project=%v write=%v, want %v", validation, projectionErr, writeErr, tc.wantErr)
			}
			if tc.wantErr != nil {
				if report.Validate() == nil || len(report.Pages) != 0 || text.Len() != 0 {
					t.Fatalf("refusal report=%v bytes=%d, want zero report and no output", report, text.Len())
				}
				return
			}
			if report.Validate() != nil || len(report.Pages) != len(book.Pages) || text.Len() == 0 {
				t.Fatalf("projection pages=%d text=%d, want %d pages and nonempty text", len(report.Pages), text.Len(), len(book.Pages))
			}
			var machine bytes.Buffer
			if err := manual.WriteJSON(&machine, report); err != nil || machine.Len() == 0 {
				t.Fatalf("machine bytes=%d err=%v, want nonempty output", machine.Len(), err)
			}
		})
	}
}

func TestManualLineSeparatorsLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		value   string
		wantErr error
	}{
		{name: "positive_internal_unicode_space", value: "left\u00a0right"},
		{name: "positive_emoji_joiner", value: "👩‍💻"},
		{name: "negative_unicode_line_separator", value: "left\u2028right", wantErr: core.ErrManualContract},
		{name: "negative_unicode_paragraph_separator", value: "left\u2029right", wantErr: core.ErrManualContract},
		{name: "positive_line_past_former_quota", value: strings.Repeat("x", manualFormerLineBytes+1)},
		{name: "neutral_empty_has_no_line", wantErr: core.ErrManualContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := manual.ParseLine(tc.value)
			if !errors.Is(err, tc.wantErr) || (tc.wantErr == nil && got.String() != tc.value) || (tc.wantErr != nil && got != "") {
				t.Fatalf("line=%q err=%v, want exact input or zero and %v", got, err, tc.wantErr)
			}
		})
	}
}

func TestManualPageReportSelfRelation(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		related bool
		wantErr error
	}{
		{name: "positive_independent_page_without_related"},
		{name: "negative_page_owns_self_relation_refusal", related: true, wantErr: core.ErrManualContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			report, err := manual.Project(namedBook(t, 1))
			if err != nil {
				t.Fatal(err)
			}
			page := report.Pages[0]
			if tc.related {
				page.Related = []manual.TopicName{page.Topic}
			}
			if err := page.Validate(); !errors.Is(err, tc.wantErr) {
				t.Fatalf("page validation=%v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestManualNilDestinationsLayerTriad(t *testing.T) {
	t.Parallel()
	book := validBook(t)
	report, err := manual.Project(book)
	if err != nil {
		t.Fatal(err)
	}
	request := manual.RenderRequest[testTopic]{Book: book, View: manual.ViewHelp, Selection: manual.Selection[testTopic]{Mode: manual.SelectionModeIndex}}
	for _, door := range []struct {
		name  string
		write func(io.Writer) error
	}{
		{name: "text", write: func(w io.Writer) error { return manual.WriteText(w, request) }},
		{name: "json", write: func(w io.Writer) error { return manual.WriteJSON(w, report) }},
	} {
		t.Run(door.name, func(t *testing.T) {
			t.Parallel()
			for _, tc := range []struct {
				name    string
				writer  io.Writer
				wantErr error
			}{
				{name: "positive_discard", writer: io.Discard},
				{name: "negative_typed_nil", writer: (*bytes.Buffer)(nil), wantErr: core.ErrManualContract},
				{name: "neutral_absent_sink", wantErr: core.ErrManualContract},
			} {
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()
					defer func() {
						if p := recover(); p != nil {
							t.Errorf("write panic=%v, want typed refusal", p)
						}
					}()
					if err := door.write(tc.writer); !errors.Is(err, tc.wantErr) {
						t.Fatalf("write error=%v, want %v", err, tc.wantErr)
					}
				})
			}
		})
	}
}

func TestManualSchemaJSONLayerTriad(t *testing.T) {
	t.Parallel()
	canonical, err := json.Marshal(manual.SchemaV1)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name    string
		data    []byte
		wantErr error
	}{
		{name: "positive_published_schema", data: canonical},
		{name: "negative_future_schema", data: []byte(`"future"`), wantErr: core.ErrManualContract},
		{name: "neutral_null_preserves_schema", data: []byte("null"), wantErr: core.ErrManualContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := manual.SchemaV1
			err := json.Unmarshal(tc.data, &got)
			if !errors.Is(err, tc.wantErr) || got != manual.SchemaV1 {
				t.Fatalf("schema=%v err=%v, want unchanged published schema and %v", got, err, tc.wantErr)
			}
		})
	}
}

func TestMachineReportRefusalLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		mutate  func(*manual.Report)
		wantErr error
	}{
		{name: "positive_complete_projection"},
		{name: "neutral_optional_related_absent", mutate: func(r *manual.Report) { r.Pages[0].Related = nil }},
		{name: "schema_unknown", mutate: func(r *manual.Report) { r.Schema = manual.SchemaUnknown }, wantErr: core.ErrManualContract},
		{name: "offering_absent", mutate: func(r *manual.Report) { r.Offering = core.Offering{} }, wantErr: core.ErrManualContract},
		{name: "title_absent", mutate: func(r *manual.Report) { r.Title = "" }, wantErr: core.ErrManualContract},
		{name: "summary_absent", mutate: func(r *manual.Report) { r.Summary = "" }, wantErr: core.ErrManualContract},
		{name: "pages_absent", mutate: func(r *manual.Report) { r.Pages = nil }, wantErr: core.ErrManualContract},
		{name: "duplicate_page_identity", mutate: func(r *manual.Report) { r.Pages[1].Topic = r.Pages[0].Topic }, wantErr: core.ErrManualContract},
		{name: "page_topic_invalid", mutate: func(r *manual.Report) { r.Pages[0].Topic = "bad..topic" }, wantErr: core.ErrManualContract},
		{name: "page_summary_absent", mutate: func(r *manual.Report) { r.Pages[0].Summary = "" }, wantErr: core.ErrManualContract},
		{name: "usage_absent", mutate: func(r *manual.Report) { r.Pages[0].Usage = nil }, wantErr: core.ErrManualContract},
		{name: "changes_absent", mutate: func(r *manual.Report) { r.Pages[0].Changes = nil }, wantErr: core.ErrManualContract},
		{name: "unchanged_absent", mutate: func(r *manual.Report) { r.Pages[0].Unchanged = nil }, wantErr: core.ErrManualContract},
		{name: "examples_absent", mutate: func(r *manual.Report) { r.Pages[0].Examples = nil }, wantErr: core.ErrManualContract},
		{name: "success_absent", mutate: func(r *manual.Report) { r.Pages[0].Outcome.Success = nil }, wantErr: core.ErrManualContract},
		{name: "refusal_absent", mutate: func(r *manual.Report) { r.Pages[0].Outcome.Refusal = nil }, wantErr: core.ErrManualContract},
		{name: "optional_prerequisite_invalid", mutate: func(r *manual.Report) { r.Pages[0].Prerequisites = []manual.Line{"x\ny"} }, wantErr: core.ErrManualContract},
		{name: "definition_invalid", mutate: func(r *manual.Report) { r.Pages[0].Definitions = []manual.Definition{{Term: "term"}} }, wantErr: core.ErrManualContract},
		{name: "relation_grammar_invalid", mutate: func(r *manual.Report) { r.Pages[0].Related = []manual.TopicName{"bad..topic"} }, wantErr: core.ErrManualContract},
		{name: "relation_not_in_report", mutate: func(r *manual.Report) { r.Pages[0].Related = []manual.TopicName{"absent"} }, wantErr: core.ErrManualContract},
		{name: "relation_self", mutate: func(r *manual.Report) { r.Pages[0].Related = []manual.TopicName{r.Pages[0].Topic} }, wantErr: core.ErrManualContract},
		{name: "relation_duplicate", mutate: func(r *manual.Report) { r.Pages[0].Related = []manual.TopicName{r.Pages[1].Topic, r.Pages[1].Topic} }, wantErr: core.ErrManualContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			report, err := manual.Project(validBook(t))
			if err != nil {
				t.Fatal(err)
			}
			if tc.mutate != nil {
				tc.mutate(&report)
			}
			validation := report.Validate()
			var sink manualBenchmarkSink
			writeErr := manual.WriteJSON(&sink, report)
			if !errors.Is(validation, tc.wantErr) || !errors.Is(writeErr, tc.wantErr) || (tc.wantErr != nil && sink.bytes != 0) || (tc.wantErr == nil && sink.bytes == 0) {
				t.Fatalf("validate=%v write=%v bytes=%d, want %v and output only on success", validation, writeErr, sink.bytes, tc.wantErr)
			}
		})
	}
}
