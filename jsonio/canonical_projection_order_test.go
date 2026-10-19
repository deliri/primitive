package jsonio

import (
	"slices"
	"testing"
	"unicode/utf16"
)

// referenceUTF16Compare is the RFC 8785 definition: compare UTF-16 encodings.
func referenceUTF16Compare(a, b string) int {
	return slices.Compare(utf16.Encode([]rune(a)), utf16.Encode([]rune(b)))
}

func TestCompareDeclaredFieldsMatchesUTF16Encoding(t *testing.T) {
	t.Parallel()
	names := []string{
		"", "a", "ab", "b", "A", "é", "€", "퟿", "", "￿", "�",
		"\U00010000", "\U0001f600", "\U0010ffff", "a\U0001f600", "a", "\xff", "\xffa", "a\xff",
		"\xed\xa0\x80", "\U0001f600", "\U0001f600",
	}
	for _, left := range names {
		for _, right := range names {
			got := compareDeclaredFields(canonicalDeclaredField{name: left}, canonicalDeclaredField{name: right})
			if want := referenceUTF16Compare(left, right); got != want {
				t.Errorf("compare(%q, %q) = %d, want UTF-16 order %d", left, right, got, want)
			}
		}
	}
}

func FuzzCompareDeclaredFieldsMatchesUTF16Encoding(f *testing.F) {
	f.Add("", "\U0001f600")
	f.Add("a", "ab")
	f.Add("\xff", "�")
	f.Fuzz(func(t *testing.T, left, right string) {
		got := compareDeclaredFields(canonicalDeclaredField{name: left}, canonicalDeclaredField{name: right})
		if want := referenceUTF16Compare(left, right); got != want {
			t.Fatalf("compare(%q, %q) = %d, want UTF-16 order %d", left, right, got, want)
		}
	})
}

func BenchmarkCompareDeclaredFields(b *testing.B) {
	left := canonicalDeclaredField{name: "nestedmember"}
	right := canonicalDeclaredField{name: "nested\U0001f600member"}
	if referenceUTF16Compare(left.name, right.name) != 1 {
		b.Fatal("fixture must exercise the supplementary/BMP ordering difference")
	}
	var got int
	b.ReportAllocs()
	for b.Loop() {
		got = compareDeclaredFields(left, right)
	}
	if got != 1 {
		b.Fatalf("compare = %d, want 1", got)
	}
}
