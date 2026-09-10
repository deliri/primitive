package core

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveTextCannotCleanAwayInvalidIngress(t *testing.T) {
	t.Parallel()
	base, err := ParseAbsolutePath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sep := string(filepath.Separator)
	cases := []struct {
		name, text string
		wantErr    error
	}{
		{name: "positive/dot remains lexical", text: "."},
		{name: "positive/parent remains lexical", text: ".."},
		{name: "positive/cancelled valid component", text: "discard" + sep + ".." + sep + "kept"},
		{name: "positive/cancelled unicode component", text: "é" + sep + ".." + sep + "kept"},
		{name: "negative/NUL in cancelled component", text: "\x00" + sep + ".." + sep + "kept", wantErr: ErrPrimitiveContract},
		{name: "negative/invalid UTF8 in cancelled component", text: "\xff" + sep + ".." + sep + "kept", wantErr: ErrPrimitiveContract},
		{name: "negative/truncated UTF8 in cancelled component", text: "\xe2\x82" + sep + ".." + sep + "kept", wantErr: ErrPrimitiveContract},
		{name: "negative/absolute NUL cannot disappear", text: base.String() + sep + "\x00" + sep + "..", wantErr: ErrPrimitiveContract},
		{name: "negative/absolute invalid UTF8 cannot disappear", text: base.String() + sep + "\xff" + sep + "..", wantErr: ErrPrimitiveContract},
	}
	for _, edge := range []struct {
		name    string
		width   int
		wantErr error
	}{
		{name: "below", width: 4096 - 1},
		{name: "exact", width: 4096},
		{name: "above", width: 4096 + 1},
	} {
		// Repeated separators deliberately normalize away. They pressure the raw
		// absence of raw extent quotas independently of the short final path.
		cases = append(cases, struct {
			name, text string
			wantErr    error
		}{name: "boundary/raw rune limit/" + edge.name, text: "." + strings.Repeat(sep, edge.width-1), wantErr: edge.wantErr})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := base.ResolveText(tc.text)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("resolve %d bytes=%v; want %v", len(tc.text), err, tc.wantErr)
			}
			if tc.wantErr != nil {
				if got != (AbsolutePath{}) {
					t.Fatalf("rejected resolution=%v; want zero", got)
				}
				return
			}
			want := filepath.Join(base.String(), tc.text)
			if filepath.IsAbs(tc.text) {
				want = filepath.Clean(tc.text)
			}
			if got.String() != want || got.Validate() != nil {
				t.Fatalf("resolved=%q; want exact Go lexical projection %q", got.String(), want)
			}
		})
	}
}
