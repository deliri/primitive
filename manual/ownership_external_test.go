package manual_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/manual"
)

func TestProjectionOwnershipLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		mutate func(*manual.Book[testTopic], *manual.Report)
	}{
		{name: "usage", mutate: func(b *manual.Book[testTopic], r *manual.Report) {
			b.Pages[0].Usage[0] = "mutated"
			r.Pages[0].Usage[0] = "mutated"
		}},
		{name: "prerequisites", mutate: func(b *manual.Book[testTopic], r *manual.Report) {
			b.Pages[0].Prerequisites[0] = "mutated"
			r.Pages[0].Prerequisites[0] = "mutated"
		}},
		{name: "changes", mutate: func(b *manual.Book[testTopic], r *manual.Report) {
			b.Pages[0].Changes[0] = "mutated"
			r.Pages[0].Changes[0] = "mutated"
		}},
		{name: "unchanged", mutate: func(b *manual.Book[testTopic], r *manual.Report) {
			b.Pages[0].Unchanged[0] = "mutated"
			r.Pages[0].Unchanged[0] = "mutated"
		}},
		{name: "definitions", mutate: func(b *manual.Book[testTopic], r *manual.Report) {
			b.Pages[0].Definitions[0].Meaning = "mutated"
			r.Pages[0].Definitions[0].Meaning = "mutated"
		}},
		{name: "examples", mutate: func(b *manual.Book[testTopic], r *manual.Report) {
			b.Pages[0].Examples[0] = "mutated"
			r.Pages[0].Examples[0] = "mutated"
		}},
		{name: "success", mutate: func(b *manual.Book[testTopic], r *manual.Report) {
			b.Pages[0].Outcome.Success[0] = "mutated"
			r.Pages[0].Outcome.Success[0] = "mutated"
		}},
		{name: "refusal", mutate: func(b *manual.Book[testTopic], r *manual.Report) {
			b.Pages[0].Outcome.Refusal[0] = "mutated"
			r.Pages[0].Outcome.Refusal[0] = "mutated"
		}},
		{name: "related", mutate: func(b *manual.Book[testTopic], r *manual.Report) {
			b.Pages[0].Related[0] = testTopicOpen
			r.Pages[0].Related[0] = r.Pages[0].Topic
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			source := validBook(t)
			got, err := manual.Project(source)
			if err != nil {
				t.Fatal(err)
			}
			detached, err := manual.Project(validBook(t))
			if err != nil {
				t.Fatal(err)
			}
			var before bytes.Buffer
			if err := manual.WriteJSON(&before, got); err != nil {
				t.Fatal(err)
			}
			// Change the source and an unrelated projection. The retained projection
			// must remain exact without relying on a JSON decoder to detach it.
			tc.mutate(&source, &detached)
			var after bytes.Buffer
			if err := manual.WriteJSON(&after, got); err != nil || !bytes.Equal(before.Bytes(), after.Bytes()) {
				t.Fatalf("retained report bytes=%d err=%v, want original %d bytes", after.Len(), err, before.Len())
			}
			untouched := validBook(t)
			changed, err := manual.Project(untouched)
			if err != nil {
				t.Fatal(err)
			}
			unrelated := validBook(t)
			tc.mutate(&unrelated, &changed)
			fresh, err := manual.Project(untouched)
			if err != nil {
				t.Fatal(err)
			}
			after.Reset()
			if err := manual.WriteJSON(&after, fresh); err != nil || !bytes.Equal(before.Bytes(), after.Bytes()) {
				t.Fatalf("source after projection mutation=%d/%v, want original %d bytes", after.Len(), err, before.Len())
			}
		})
	}
}

type manualWindowSink struct {
	bytes   int64
	largest int
}

func (w *manualWindowSink) Write(data []byte) (int, error) {
	w.bytes += int64(len(data))
	w.largest = max(w.largest, len(data))
	return len(data), nil
}
func TestTextExtentLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		size      int
		malformed bool
		wantErr   error
	}{
		{name: "below_window", size: manualStreamWindow - 1},
		{name: "exact_window", size: manualStreamWindow},
		{name: "above_window", size: manualStreamWindow + 1},
		{name: "many_windows_and_partial_tail", size: 1<<20 + 1},
		{name: "late_malformed_line_emits_nothing", size: 1<<20 + 1, malformed: true, wantErr: core.ErrManualContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			book := namedBook(t, 1)
			ordinary := book.Pages[0].Summary
			request := manual.RenderRequest[namedTopic]{Book: book, View: manual.ViewHelp, Selection: manual.Selection[namedTopic]{Mode: manual.SelectionModeTopic, Topic: book.Pages[0].Topic}}
			var baseline manualWindowSink
			if err := manual.WriteText(&baseline, request); err != nil {
				t.Fatal(err)
			}
			book.Pages[0].Summary = manual.Line(strings.Repeat("x", tc.size))
			if tc.malformed {
				book.Pages[0].Summary += "\n"
			}
			request.Book = book
			var got manualWindowSink
			err := manual.WriteText(&got, request)
			want := baseline.bytes - int64(len(ordinary)) + int64(tc.size)
			if tc.wantErr != nil {
				want = 0
			}
			if !errors.Is(err, tc.wantErr) || got.bytes != want || got.largest > manualStreamWindow {
				t.Fatalf("bytes=%d largest-write=%d error=%v, want bytes=%d window<=%d error=%v", got.bytes, got.largest, err, want, manualStreamWindow, tc.wantErr)
			}
		})
	}
}

func BenchmarkManualExtent(b *testing.B) {
	b.ReportAllocs()
	for _, tc := range []struct {
		name string
		size int
	}{
		{name: "text_1KiB", size: 1024},
		{name: "text_1MiB", size: 1 << 20},
	} {
		b.Run(tc.name, func(b *testing.B) {
			book := namedBook(b, 1)
			book.Pages[0].Summary = manual.Line(strings.Repeat("x", tc.size))
			request := manual.RenderRequest[namedTopic]{Book: book, View: manual.ViewHelp, Selection: manual.Selection[namedTopic]{Mode: manual.SelectionModeTopic, Topic: book.Pages[0].Topic}}
			var sink manualWindowSink
			if err := manual.WriteText(&sink, request); err != nil || sink.bytes < int64(tc.size) {
				b.Fatalf("setup=%d/%v, want complete line extent=%d", sink.bytes, err, tc.size)
			}
			want := sink.bytes
			b.ReportAllocs()
			b.SetBytes(want)
			for b.Loop() {
				sink.bytes = 0
				sink.largest = 0
				if err := manual.WriteText(&sink, request); err != nil || sink.bytes != want || sink.largest > manualStreamWindow {
					b.Fatalf("text=%d window=%d error=%v, want bytes=%d bounded writes", sink.bytes, sink.largest, err, want)
				}
			}
		})
	}
}
