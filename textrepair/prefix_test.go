package textrepair

import (
	"errors"
	"math"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
)

func TestUTF8PrefixLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, source, want string
		budget             uint64
		wantErr            error
	}{
		{name: "valid multiwidth prefix fits exactly", source: "aé界😀", budget: 10, want: "aé界😀"},
		{name: "crossing rune cannot be skipped for smaller suffix", source: "éa", budget: 1},
		{name: "malformed prefix repaired before budget", source: "\xffaé", budget: 3, want: "aé"},
		{name: "genuine replacement rune survives", source: "\xff�x", budget: 3, want: "�"},
		{name: "zero budget emits no source", source: "private", budget: 0},
		{name: "empty source at extreme budget remains empty", budget: math.MaxInt64},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := Prefix(Request{Source: tc.source, MaximumBytes: prefixBudget(t, tc.budget)})
			if !errors.Is(err, tc.wantErr) || got != tc.want {
				t.Fatalf("Prefix() = %q/error %v, want %q/%v", got, err, tc.want, tc.wantErr)
			}
		})
	}
}

func TestUTF8PrefixExhaustiveSingleByteDomain(t *testing.T) {
	t.Parallel()
	for value := 0; value < 256; value++ {
		for budget := uint64(0); budget <= 2; budget++ {
			source := string([]byte{byte(value)})
			want := ""
			if value < utf8.RuneSelf && budget > 0 {
				want = source
			}
			got, err := Prefix(Request{Source: source, MaximumBytes: prefixBudget(t, budget)})
			if err != nil || got != want {
				t.Fatalf("byte %d budget %d = %q/error %v, want %q/nil", value, budget, got, err, want)
			}
		}
	}
}

func FuzzUTF8PrefixSemanticOrder(f *testing.F) {
	seed, err := Prefix(Request{Source: "aé界😀�", MaximumBytes: prefixBudget(f, 13)})
	if err != nil {
		f.Fatalf("seed error = %v, want nil", err)
	}
	f.Add(seed, uint64(13))
	f.Add("\xff\xc0a�", uint64(4))
	f.Fuzz(func(t *testing.T, source string, budget uint64) {
		extent, err := core.NewByteLength(budget)
		if err != nil {
			if !errors.Is(err, core.ErrNumericOverflow) {
				t.Fatalf("extent error = %v, want %v", err, core.ErrNumericOverflow)
			}
			return
		}
		got, err := Prefix(Request{Source: source, MaximumBytes: extent})
		if err != nil {
			t.Fatalf("repair error = %v, want nil", err)
		}
		// Independent range iterator pins exact source order and malformed-byte removal.
		var want strings.Builder
		for index, r := range source {
			if r == utf8.RuneError && !strings.HasPrefix(source[index:], "�") {
				continue
			}
			if uint64(want.Len()+utf8.RuneLen(r)) > budget {
				break
			}
			want.WriteRune(r)
		}
		if got != want.String() || !utf8.ValidString(got) || uint64(len(got)) > budget {
			t.Fatalf("repair = %q, want bounded ordered %q", got, want.String())
		}
		second, err := Prefix(Request{Source: got, MaximumBytes: extent})
		if err != nil || second != got {
			t.Fatalf("second repair = %q/error %v, want %q/nil", second, err, got)
		}
	})
}

func prefixBudget(t testing.TB, bytes uint64) core.ByteLength {
	t.Helper()
	length, err := core.NewByteLength(bytes)
	if err != nil {
		t.Fatalf("NewByteLength(%d) error = %v, want nil", bytes, err)
	}
	return length
}
