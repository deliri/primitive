package tailnetconfig_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/tailnetconfig"
	"tailscale.com/tailcfg"
)

func TestTagGrammarMatchesPinnedProvider(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		input   string
		wantErr error
	}{
		{"minimum lower-case letter", "tag:a", nil},
		{"minimum upper-case letter is a distinct provider tag", "tag:A", nil},
		{"provider permits trailing hyphen", "tag:a-", nil},
		{"provider permits mixed case without repair", "tag:Production-A", nil},
		{"digits after initial letter remain exact", "tag:a0", nil},
		{"digit cannot begin provider tag", "tag:0", core.ErrTailnetContract},
		{"hyphen cannot begin provider tag", "tag:-a", core.ErrTailnetContract},
		{"underscore is not provider punctuation", "tag:a_b", core.ErrTailnetContract},
		{"unicode lookalike cannot become provider authority", "tag:а", core.ErrTailnetContract},
		{"empty suffix cannot become provider authority", "tag:", core.ErrTailnetContract},
		{"missing prefix cannot become provider authority", "a", core.ErrTailnetContract},
		{"one below local suffix bound", tailnetconfig.TagPrefix + strings.Repeat("A", tailnetconfig.TagNameMaximumBytes-1), nil},
		{"exact local suffix bound", tailnetconfig.TagPrefix + strings.Repeat("A", tailnetconfig.TagNameMaximumBytes), nil},
		{"above local suffix bound remains a local refusal", tailnetconfig.TagPrefix + strings.Repeat("A", tailnetconfig.TagNameMaximumBytes+1), core.ErrTailnetContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := tailnetconfig.ParseTag(tc.input)
			providerErr := tailcfg.CheckTag(tc.input)
			wantProviderAdmitted := tc.wantErr == nil || len(tc.input) > len(tailnetconfig.TagPrefix)+tailnetconfig.TagNameMaximumBytes
			if (providerErr == nil) != wantProviderAdmitted {
				t.Fatalf("pinned provider admission = %v, want admitted=%t", providerErr, wantProviderAdmitted)
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("ParseTag() = %v, want %v", err, tc.wantErr)
			}
			if tc.wantErr == nil && got.String() != tc.input {
				t.Fatalf("tag spelling = %q, want %q", got.String(), tc.input)
			}
			if tc.wantErr != nil && got != "" {
				t.Fatalf("refused tag = %q, want zero", got)
			}
		})
	}
}
