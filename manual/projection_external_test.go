package manual_test

import (
	"bytes"
	json "encoding/json/v2"
	"slices"
	"testing"

	"github.com/deliri/primitive/v2026/manual"
)

func TestMachineProjectionLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		optional bool
	}{
		{name: "positive_complete_projection", optional: true},
		{name: "neutral_optional_sections_absent"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			book := validBook(t)
			if !tc.optional {
				for i := range book.Pages {
					book.Pages[i].Prerequisites = nil
					book.Pages[i].Definitions = nil
					book.Pages[i].Related = nil
				}
			}
			want := manual.Report{Schema: manual.SchemaV1, Offering: book.Offering, Title: book.Title, Summary: book.Summary}
			for _, p := range book.Pages {
				related := make([]manual.TopicName, 0, len(p.Related))
				for _, topic := range p.Related {
					related = append(related, topic.ManualTopic())
				}
				want.Pages = append(want.Pages, manual.PageReport{
					Topic: p.Topic.ManualTopic(), Summary: p.Summary, Usage: p.Usage, Prerequisites: p.Prerequisites,
					Changes: p.Changes, Unchanged: p.Unchanged, Definitions: p.Definitions, Examples: p.Examples,
					Outcome: p.Outcome, Related: related,
				})
			}
			got, err := manual.Project(book)
			if err != nil || got.Schema != want.Schema || got.Offering != want.Offering || got.Title != want.Title || got.Summary != want.Summary || len(got.Pages) != len(want.Pages) {
				t.Fatalf("projection=%#v/%v, want exact typed fields %#v", got, err, want)
			}
			for i, page := range got.Pages {
				expected := want.Pages[i]
				if page.Topic != expected.Topic || page.Summary != expected.Summary || !slices.Equal(page.Usage, expected.Usage) || !slices.Equal(page.Prerequisites, expected.Prerequisites) || !slices.Equal(page.Changes, expected.Changes) || !slices.Equal(page.Unchanged, expected.Unchanged) || !slices.Equal(page.Definitions, expected.Definitions) || !slices.Equal(page.Examples, expected.Examples) || !slices.Equal(page.Outcome.Success, expected.Outcome.Success) || !slices.Equal(page.Outcome.Refusal, expected.Outcome.Refusal) || !slices.Equal(page.Related, expected.Related) {
					t.Fatalf("page[%d]=%#v, want exact typed fields %#v", i, page, expected)
				}
			}
			var encoded bytes.Buffer
			if err := manual.WriteJSON(&encoded, got); err != nil {
				t.Fatal(err)
			}
			var decoded manual.Report
			if err := json.Unmarshal(encoded.Bytes(), &decoded); err != nil {
				t.Fatal(err)
			}
			if err := decoded.Validate(); err != nil {
				t.Fatal(err)
			}
			var second bytes.Buffer
			if err := manual.WriteJSON(&second, decoded); err != nil || !bytes.Equal(encoded.Bytes(), second.Bytes()) {
				t.Fatalf("roundtrip=%q/%v, want original %q", second.Bytes(), err, encoded.Bytes())
			}
		})
	}
}
