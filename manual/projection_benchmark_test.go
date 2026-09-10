package manual_test

import (
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/manual"
)

type manualBenchmarkSink struct{ bytes int64 }

func (w *manualBenchmarkSink) Write(data []byte) (int, error) {
	w.bytes += int64(len(data))
	return len(data), nil
}

func BenchmarkManualBook(b *testing.B) {
	b.ReportAllocs()
	book := validBook(b)
	b.Run("validate", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if err := book.Validate(); err != nil {
				b.Fatalf("book validation=%v, want nil", err)
			}
		}
	})
	b.Run("project", func(b *testing.B) {
		b.ReportAllocs()
		var got manual.Report
		for b.Loop() {
			var err error
			got, err = manual.Project(book)
			if err != nil {
				b.Fatalf("projection=%v, want nil", err)
			}
		}
		if got.Validate() != nil || len(got.Pages) != len(book.Pages) || got.Pages[0].Summary != book.Pages[0].Summary {
			b.Fatalf("projected pages=%d, want exact book pages=%d", len(got.Pages), len(book.Pages))
		}
	})
}

func BenchmarkManualText(b *testing.B) {
	b.ReportAllocs()
	for _, tc := range []struct {
		name      string
		view      manual.View
		selection manual.Selection[testTopic]
		wide      bool
	}{
		{name: "index", view: manual.ViewHelp, selection: manual.Selection[testTopic]{Mode: manual.SelectionModeIndex}},
		{name: "help", view: manual.ViewHelp, selection: manual.Selection[testTopic]{Mode: manual.SelectionModeTopic, Topic: testTopicOpen}},
		{name: "manual", view: manual.ViewManual, selection: manual.Selection[testTopic]{Mode: manual.SelectionModeTopic, Topic: testTopicOpen}},
		{name: "manual_1KiB_line", view: manual.ViewManual, selection: manual.Selection[testTopic]{Mode: manual.SelectionModeTopic, Topic: testTopicOpen}, wide: true},
	} {
		b.Run(tc.name, func(b *testing.B) {
			book := validBook(b)
			if tc.wide {
				book.Pages[0].Summary = manual.Line(strings.Repeat("x", 1024))
			}
			request := manual.RenderRequest[testTopic]{Book: book, View: tc.view, Selection: tc.selection}
			var sink manualBenchmarkSink
			if err := manual.WriteText(&sink, request); err != nil || sink.bytes == 0 {
				b.Fatalf("setup text bytes=%d error=%v, want nonempty output", sink.bytes, err)
			}
			want := sink.bytes
			b.ReportAllocs()
			b.SetBytes(want)
			for b.Loop() {
				sink.bytes = 0
				if err := manual.WriteText(&sink, request); err != nil {
					b.Fatalf("text write=%v, want nil", err)
				}
				if sink.bytes != want {
					b.Fatalf("text bytes=%d, want %d", sink.bytes, want)
				}
			}
		})
	}
}

func BenchmarkManualJSON(b *testing.B) {
	b.ReportAllocs()
	report, err := manual.Project(validBook(b))
	if err != nil {
		b.Fatalf("project setup=%v, want nil", err)
	}
	var sink manualBenchmarkSink
	if err := manual.WriteJSON(&sink, report); err != nil || sink.bytes == 0 {
		b.Fatalf("setup JSON bytes=%d error=%v, want nonempty output", sink.bytes, err)
	}
	want := sink.bytes
	b.SetBytes(want)
	for b.Loop() {
		sink.bytes = 0
		if err := manual.WriteJSON(&sink, report); err != nil {
			b.Fatalf("JSON write=%v, want nil", err)
		}
		if sink.bytes != want {
			b.Fatalf("JSON bytes=%d, want %d", sink.bytes, want)
		}
	}
}
