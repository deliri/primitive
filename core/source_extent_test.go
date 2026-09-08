package core

import (
	"errors"
	"strings"
	"testing"
)

func TestSourcePathExtentDoesNotReplaceCanonicalValidation(t *testing.T) {
	t.Parallel()
	prefix := strings.Repeat("segment/", 512)
	for _, tc := range []struct {
		name, path string
		want       error
	}{
		{name: "long canonical path", path: prefix + "value.go"},
		{name: "long path cannot escape at tail", path: prefix + "../value.go", want: ErrPrimitiveContract},
		{name: "long path cannot hide binary tail", path: prefix + "value.go\x00", want: ErrPrimitiveContract},
		{name: "long path cannot hide invalid encoding", path: prefix + "\xff.go", want: ErrPrimitiveContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseSourcePath(tc.path)
			if !errors.Is(err, tc.want) {
				t.Fatalf("source identity = %v, want %v", err, tc.want)
			}
			if tc.want != nil {
				if got.String() != "" {
					t.Fatalf("refused source path = %q, want empty", got.String())
				}
				return
			}
			encoded, err := got.MarshalJSON()
			if err != nil {
				t.Fatal(err)
			}
			var decoded SourcePath
			if err := decoded.UnmarshalJSON(encoded); err != nil || decoded.String() != tc.path {
				t.Fatalf("path lost canonical tail in transport: %v", err)
			}
		})
	}
}
