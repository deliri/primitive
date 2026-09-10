package manual

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

type renderTopic uint8

const (
	renderUnknown renderTopic = iota
	renderFirst
	renderSecond
)

func (v renderTopic) Validate() error {
	if v != renderFirst && v != renderSecond {
		return core.ErrManualContract
	}
	return nil
}
func (v renderTopic) ManualTopic() TopicName {
	if v == renderFirst {
		return "first"
	}
	if v == renderSecond {
		return "second"
	}
	return ""
}

func renderBook(t testing.TB) Book[renderTopic] {
	t.Helper()
	book := Book[renderTopic]{Title: "Guide", Summary: "Overview.", Offering: core.Offering{Token: "manual-fixture"}}
	for _, topic := range []renderTopic{renderFirst, renderSecond} {
		book.Pages = append(book.Pages, Page[renderTopic]{
			Topic: topic, Summary: "Summary.", Usage: []Line{"use"}, Prerequisites: []Line{"ready"},
			Changes: []Line{"writes"}, Unchanged: []Line{"keeps"},
			Definitions: []Definition{{Term: "item", Meaning: "one"}}, Examples: []Line{"example"},
			Outcome: Outcome{Success: []Line{"yes"}, Refusal: []Line{"no"}},
		})
	}
	book.Pages[0].Related = []renderTopic{renderSecond}
	if err := book.Validate(); err != nil {
		t.Fatalf("fixture=%v, want valid book", err)
	}
	return book
}

func TestTextLayoutLayerTriad(t *testing.T) {
	t.Parallel()
	help := "first\n\nSummary.\n\n" + headingUsage + "\n\n- use\n\n" + headingSuccess + "\n\n- yes\n\n" + headingRefusal + "\n\n- no\n\n"
	full := help + headingPrerequisites + "\n\n- ready\n\n" + headingChanges + "\n\n- writes\n\n" + headingUnchanged + "\n\n- keeps\n\n" + headingExamples + "\n\n- example\n\n" + headingTerms + "\n\n- item: one\n\n" + headingRelated + "\n\n- second\n"
	for _, tc := range []struct {
		name           string
		selection      Selection[renderTopic]
		view           View
		optionalAbsent bool
		topicAbsent    bool
		reverse        bool
		want           string
		wantErr        error
	}{
		{name: "index_preserves_declared_order", selection: Selection[renderTopic]{Mode: SelectionModeIndex}, view: ViewHelp, want: "Guide\n\nOverview.\n\n" + headingTopics + "\n\n- first: Summary.\n- second: Summary.\n"},
		{name: "index_preserves_reversed_declaration", selection: Selection[renderTopic]{Mode: SelectionModeIndex}, view: ViewHelp, reverse: true, want: "Guide\n\nOverview.\n\n" + headingTopics + "\n\n- second: Summary.\n- first: Summary.\n"},
		{name: "valid_topic_absent_from_book", selection: Selection[renderTopic]{Mode: SelectionModeTopic, Topic: renderSecond}, view: ViewHelp, topicAbsent: true, wantErr: core.ErrManualContract},
		{name: "help_omits_full_manual_sections", selection: Selection[renderTopic]{Mode: SelectionModeTopic, Topic: renderFirst}, view: ViewHelp, want: help},
		{name: "manual_emits_each_section_once", selection: Selection[renderTopic]{Mode: SelectionModeTopic, Topic: renderFirst}, view: ViewManual, want: full},
		{name: "neutral_optional_absence_creates_no_headings", selection: Selection[renderTopic]{Mode: SelectionModeTopic, Topic: renderFirst}, view: ViewManual, optionalAbsent: true, want: help + headingChanges + "\n\n- writes\n\n" + headingUnchanged + "\n\n- keeps\n\n" + headingExamples + "\n\n- example\n\n"},
		{name: "index_cannot_carry_a_topic", selection: Selection[renderTopic]{Mode: SelectionModeIndex, Topic: renderFirst}, view: ViewHelp, wantErr: core.ErrManualContract},
		{name: "topic_selection_cannot_omit_identity", selection: Selection[renderTopic]{Mode: SelectionModeTopic}, view: ViewHelp, wantErr: core.ErrManualContract},
		{name: "unknown_view_emits_nothing", selection: Selection[renderTopic]{Mode: SelectionModeIndex}, wantErr: core.ErrManualContract},
		{name: "unknown_selection_emits_nothing", view: ViewHelp, wantErr: core.ErrManualContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			book := renderBook(t)
			if tc.reverse {
				book.Pages[0], book.Pages[1] = book.Pages[1], book.Pages[0]
			}
			if tc.topicAbsent {
				book.Pages = book.Pages[:1]
				book.Pages[0].Related = nil
			}
			if tc.optionalAbsent {
				book.Pages[0].Prerequisites = nil
				book.Pages[0].Definitions = nil
				book.Pages[0].Related = nil
			}
			var got bytes.Buffer
			err := WriteText(&got, RenderRequest[renderTopic]{Book: book, View: tc.view, Selection: tc.selection})
			if !errors.Is(err, tc.wantErr) || got.String() != tc.want {
				t.Fatalf("text=%q error=%v, want %q/%v", got.String(), err, tc.want, tc.wantErr)
			}
		})
	}
}

type prefixWriter struct {
	got          bytes.Buffer
	remaining    int
	err          error
	calls        int
	failed       bool
	afterFailure bool
}

func (w *prefixWriter) Write(data []byte) (int, error) {
	if w.failed {
		w.afterFailure = true
	}
	w.calls++
	n := min(w.remaining, len(data))
	_, _ = w.got.Write(data[:n]) // bytes.Buffer.Write cannot fail.
	w.remaining -= n
	if n < len(data) || w.remaining == 0 && w.err != nil {
		w.failed = true
		return n, w.err
	}
	return n, nil
}

type invalidCountWriter struct {
	count int
	err   error
}

func (w invalidCountWriter) Write(data []byte) (int, error) {
	if w.count < 0 {
		return w.count, w.err
	}
	return len(data) + 1, w.err
}

func TestWriterBackpressureLayerTriad(t *testing.T) {
	t.Parallel()
	book := renderBook(t)
	report, err := Project(book)
	if err != nil {
		t.Fatal(err)
	}
	for _, door := range []struct {
		name  string
		write func(io.Writer) error
	}{
		{name: "text", write: func(w io.Writer) error {
			return WriteText(w, RenderRequest[renderTopic]{Book: book, View: ViewManual, Selection: Selection[renderTopic]{Mode: SelectionModeTopic, Topic: renderFirst}})
		}},
		{name: "json", write: func(w io.Writer) error { return WriteJSON(w, report) }},
	} {
		t.Run(door.name, func(t *testing.T) {
			t.Parallel()
			var complete bytes.Buffer
			if err := door.write(&complete); err != nil || complete.Len() == 0 {
				t.Fatalf("fixture bytes=%d error=%v, want nonempty output", complete.Len(), err)
			}
			for _, tc := range []struct {
				name  string
				cause error
			}{
				{name: "short_count_without_native_error"},
				{name: "native_failure_preserves_exact_prefix", cause: io.ErrClosedPipe},
			} {
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()
					for count := 0; count <= complete.Len(); count++ {
						w := prefixWriter{remaining: count, err: tc.cause}
						err := door.write(&w)
						wantErr := tc.cause
						if count < complete.Len() && wantErr == nil {
							wantErr = io.ErrShortWrite
						}
						if !errors.Is(err, wantErr) || (wantErr != nil && !errors.Is(err, core.ErrManualWrite)) || !bytes.Equal(w.got.Bytes(), complete.Bytes()[:count]) || w.afterFailure {
							t.Fatalf("prefix=%d got=%d error=%v writes-after-failure=%t, want exact prefix and %v", count, w.got.Len(), err, w.afterFailure, wantErr)
						}
					}
				})
			}
			for _, tc := range []struct {
				name  string
				count int
				cause error
			}{
				{name: "negative_count", count: -1},
				{name: "excess_count", count: 1},
				{name: "excess_count_with_native_failure", count: 1, cause: io.ErrClosedPipe},
			} {
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()
					err := door.write(invalidCountWriter{count: tc.count, err: tc.cause})
					if !errors.Is(err, core.ErrManualWrite) || !errors.Is(err, io.ErrShortWrite) || (tc.cause != nil && !errors.Is(err, tc.cause)) {
						t.Fatalf("writer error=%v, want write and short-count identity with cause %v", err, tc.cause)
					}
				})
			}
		})
	}
}

func TestClosedManualEnums(t *testing.T) {
	t.Parallel()
	for raw := range 256 {
		schema := Schema(raw)
		want := schema == SchemaV1
		data, err := schema.MarshalJSON()
		if schema.IsValid() != want || (schema.Validate() == nil) != want || (err == nil) != want {
			t.Fatalf("schema=%d valid=%t marshal=%v, want admitted=%t", raw, schema.IsValid(), err, want)
		}
		if !want {
			if data != nil || !errors.Is(err, core.ErrJSONContract) || !errors.Is(err, core.ErrManualContract) {
				t.Fatalf("schema=%d bytes=%q error=%v, want nil typed refusal", raw, data, err)
			}
		} else if schema.String() != SchemaV1Token {
			t.Fatalf("schema token=%q, want %q", schema.String(), SchemaV1Token)
		}
		view := View(raw)
		wantView := view == ViewHelp || view == ViewManual
		viewBytes, viewErr := view.MarshalJSON()
		if view.IsValid() != wantView || (viewErr == nil) != wantView {
			t.Fatalf("view=%d error=%v, want admitted=%t", raw, viewErr, wantView)
		}
		if !wantView && (!errors.Is(viewErr, core.ErrManualContract) || viewBytes != nil) {
			t.Fatalf("view refusal=%q/%v, want nil and typed refusal", viewBytes, viewErr)
		}
		mode := SelectionMode(raw)
		wantMode := mode == SelectionModeIndex || mode == SelectionModeTopic
		modeBytes, modeErr := mode.MarshalJSON()
		if mode.IsValid() != wantMode || (modeErr == nil) != wantMode {
			t.Fatalf("mode=%d error=%v, want admitted=%t", raw, modeErr, wantMode)
		}
		if !wantMode && (!errors.Is(modeErr, core.ErrManualContract) || modeBytes != nil) {
			t.Fatalf("mode refusal=%q/%v, want nil and typed refusal", modeBytes, modeErr)
		}
	}
}

func TestManualEnumNilReceivers(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		decode func([]byte) error
	}{
		{name: "schema", decode: (*Schema)(nil).UnmarshalJSON},
		{name: "view", decode: (*View)(nil).UnmarshalJSON},
		{name: "selection_mode", decode: (*SelectionMode)(nil).UnmarshalJSON},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if err := tc.decode(nil); !errors.Is(err, core.ErrManualContract) || !errors.Is(err, core.ErrJSONContract) {
				t.Fatalf("nil receiver=%v, want typed JSON/manual refusal", err)
			}
		})
	}
}

// Each primitive heading is a compiler-owned constant; the expected layout
// above independently specifies ordering, spacing and omission.
func TestTextWindowFailureLayerTriad(t *testing.T) {
	t.Parallel()
	window := bufio.NewWriter(io.Discard).Size()
	book := renderBook(t)
	book.Pages[0].Summary = Line(strings.Repeat("x", window*2+1))
	request := RenderRequest[renderTopic]{Book: book, View: ViewManual, Selection: Selection[renderTopic]{Mode: SelectionModeTopic, Topic: renderFirst}}
	var complete bytes.Buffer
	if err := WriteText(&complete, request); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		prefix int
		cause  error
	}{
		{name: "before_first_flush", prefix: window - 1},
		{name: "at_first_flush", prefix: window},
		{name: "after_first_flush", prefix: window + 1},
		{name: "second_flush_native_failure", prefix: window * 2, cause: io.ErrClosedPipe},
		{name: "final_flush_native_failure", prefix: complete.Len(), cause: io.ErrClosedPipe},
		{name: "neutral_complete_output", prefix: complete.Len()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			destination := prefixWriter{remaining: tc.prefix, err: tc.cause}
			err := WriteText(&destination, request)
			wantErr := tc.cause
			if wantErr == nil && tc.prefix < complete.Len() {
				wantErr = io.ErrShortWrite
			}
			if !errors.Is(err, wantErr) || (wantErr != nil && !errors.Is(err, core.ErrManualWrite)) || destination.afterFailure || !bytes.Equal(destination.got.Bytes(), complete.Bytes()[:tc.prefix]) {
				t.Fatalf("prefix=%d output=%d writes-after-failure=%t error=%v, want exact prefix and %v", tc.prefix, destination.got.Len(), destination.afterFailure, err, wantErr)
			}
		})
	}
}
