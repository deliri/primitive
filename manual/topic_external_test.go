package manual_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/manual"
)

func TestTopicSegmentsLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		text    string
		wantErr error
	}{
		{name: "single command", text: "compile"},
		{name: "two command segments", text: "work.list"},
		{name: "three command segments", text: "anvil.file.run"},
		{name: "hyphenated segments", text: "work-item.read-all"},
		{name: "numeric segment", text: "api.v2"},
		{name: "exact topic bound", text: strings.Repeat("a", manual.MaximumTopicBytes)},
		{name: "empty", wantErr: core.ErrManualContract},
		{name: "leading dot", text: ".work", wantErr: core.ErrManualContract},
		{name: "trailing dot", text: "work.", wantErr: core.ErrManualContract},
		{name: "empty segment", text: "work..list", wantErr: core.ErrManualContract},
		{name: "leading segment hyphen", text: "work.-list", wantErr: core.ErrManualContract},
		{name: "trailing segment hyphen", text: "work-.list", wantErr: core.ErrManualContract},
		{name: "repeated hyphen", text: "work--list", wantErr: core.ErrManualContract},
		{name: "uppercase is not repaired", text: "Work.list", wantErr: core.ErrManualContract},
		{name: "slash is not a topic separator", text: "work/list", wantErr: core.ErrManualContract},
		{name: "space is not trimmed", text: "work.list ", wantErr: core.ErrManualContract},
		{name: "over topic bound", text: strings.Repeat("a", manual.MaximumTopicBytes+1), wantErr: core.ErrManualContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := manual.NewTopicName(tc.text)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("NewTopicName() error = %v, want %v", err, tc.wantErr)
			}
			if err == nil && (got.String() != tc.text || got.Validate() != nil) {
				t.Fatalf("NewTopicName() = %q, want exact admitted text %q", got, tc.text)
			}
			if err != nil && got != "" {
				t.Fatalf("NewTopicName(refused) = %q, want zero", got)
			}
		})
	}
}
