package process_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/process"
)

func TestFuzzVectorCustodyLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		values []string
	}{
		{name: "neutral/no arguments stays zero", values: nil},
		{name: "boundary/one empty argument stays one", values: []string{""}},
		{name: "boundary/adjacent empty arguments stay distinct", values: []string{"", "", ""}},
		{name: "positive/ordered arguments stay separate", values: []string{"first", "second"}},
		{name: "negative/former separator cannot split an argument", values: []string{"before\xffafter"}},
		{name: "negative/repeated separator cannot invent arguments", values: []string{"\xff", "\xff\xff"}},
		{name: "negative/NUL reaches production unchanged", values: []string{"A=\x00", "tail"}},
		{name: "boundary/invalid UTF8 remains raw bytes", values: []string{"\xc0\x80", "\xed\xa0\x80"}},
		{name: "boundary/overlong argument is not truncated", values: []string{strings.Repeat("x", int(process.ArgumentMaximumBytes)+1)}},
		{name: "boundary/too many arguments are not dropped", values: make([]string, int(process.ArgumentCountMaximum)+1)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			encoded, err := joinProcessFuzzVector(tc.values...)
			if err != nil {
				t.Fatal(err)
			}
			got := processFuzzVector(encoded)
			if !slices.Equal(got, tc.values) || len(got) != len(tc.values) {
				t.Fatalf("seed framing changed arguments: got lengths %d, want %d", len(got), len(tc.values))
			}
		})
	}
}
