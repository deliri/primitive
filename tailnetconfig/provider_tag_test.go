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
		wantErr error
		name    string
		input   string
	}{
		{name: "minimum lower-case letter", input: "tag:a", wantErr: nil},
		{name: "minimum upper-case letter is a distinct provider tag", input: "tag:A", wantErr: nil},
		{name: "provider permits trailing hyphen", input: "tag:a-", wantErr: nil},
		{name: "provider permits mixed case without repair", input: "tag:Production-A", wantErr: nil},
		{name: "digits after initial letter remain exact", input: "tag:a0", wantErr: nil},
		{name: "digit cannot begin provider tag", input: "tag:0", wantErr: core.ErrTailnetContract},
		{name: "hyphen cannot begin provider tag", input: "tag:-a", wantErr: core.ErrTailnetContract},
		{name: "underscore is not provider punctuation", input: "tag:a_b", wantErr: core.ErrTailnetContract},
		{name: "unicode lookalike cannot become provider authority", input: "tag:а", wantErr: core.ErrTailnetContract},
		{name: "empty suffix cannot become provider authority", input: "tag:", wantErr: core.ErrTailnetContract},
		{name: "missing prefix cannot become provider authority", input: "a", wantErr: core.ErrTailnetContract},
		{name: "one below local suffix bound", input: tailnetconfig.TagPrefix + strings.Repeat("A", tailnetconfig.TagNameMaximumBytes-1), wantErr: nil},
		{name: "exact local suffix bound", input: tailnetconfig.TagPrefix + strings.Repeat("A", tailnetconfig.TagNameMaximumBytes), wantErr: nil},
		{name: "above local suffix bound remains a local refusal", input: tailnetconfig.TagPrefix + strings.Repeat("A", tailnetconfig.TagNameMaximumBytes+1), wantErr: core.ErrTailnetContract},
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
