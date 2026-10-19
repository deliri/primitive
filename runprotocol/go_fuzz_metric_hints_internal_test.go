package runprotocol

import (
	"bytes"
	"testing"
)

// TestContainsASCIIFoldedMatchesExplicitASCIIFolding proves the in-place
// comparison equals lowering only ASCII capitals and then searching: case
// pairs outside ASCII never fold, and matches at either edge are found.
func TestContainsASCIIFoldedMatchesExplicitASCIIFolding(t *testing.T) {
	t.Parallel()
	const literal = "failing input"
	cases := []struct {
		window string
		want   bool
	}{
		{window: "failing input", want: true},
		{window: "FAILING INPUT written to x", want: true},
		{window: "prefix FaIlInG iNpUt", want: true},
		{window: "failing inpu", want: false},
		{window: "failing\tinput", want: false},
		{window: "failİng input", want: false},
		{window: "Kailing input failing input", want: true},
		{window: "ffailing input", want: true},
		{window: "", want: false},
	}
	for _, tc := range cases {
		folded := []byte(tc.window)
		for index, value := range folded {
			if value >= 'A' && value <= 'Z' {
				folded[index] = value + 'a' - 'A'
			}
		}
		reference := bytes.Contains(folded, []byte(literal))
		got := containsASCIIFolded([]byte(tc.window), literal)
		if got != tc.want || got != reference {
			t.Fatalf("containsASCIIFolded(%q) = %t, want %t (reference %t)", tc.window, got, tc.want, reference)
		}
	}
}
