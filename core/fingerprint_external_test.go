package core_test

import (
	"hash/fnv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestFNV1a64TextPartsKeepsNativeVectorsAndExactFraming(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		parts []string
		want  uint64
	}{
		{name: "absent parts are empty input", want: 0xcbf29ce484222325},
		{name: "empty part is empty input", parts: []string{""}, want: 0xcbf29ce484222325},
		{name: "known hello vector", parts: []string{"hello"}, want: 0xa430d84680aabd0b},
		{name: "part boundaries do not alter bytes", parts: []string{"h", "e", "", "ll", "o"}, want: 0xa430d84680aabd0b},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := core.FNV1a64TextParts(tc.parts...); got != tc.want {
				t.Fatalf("fingerprint = %x, want %x", got, tc.want)
			}
		})
	}
	for _, text := range []string{"\x00", "\xff", "a\x00b", "b\x00a", "α/本", strings.Repeat("x", 4095), strings.Repeat("x", 4096), strings.Repeat("x", 4097)} {
		t.Run(text[:min(len(text), 12)], func(t *testing.T) {
			t.Parallel()
			oracle := fnv.New64a()
			for _, value := range []byte(text) {
				_, _ = oracle.Write([]byte{value})
			}
			for split := 0; split <= len(text); split++ {
				if got := core.FNV1a64TextParts(text[:split], "", text[split:]); got != oracle.Sum64() {
					t.Fatalf("split %d fingerprint = %x, want %x", split, got, oracle.Sum64())
				}
			}
		})
	}
}

func FuzzFNV1a64TextPartsNativeConcatenation(f *testing.F) {
	f.Add("", "", "")
	f.Add("a", "\x00", "b")
	f.Add("α", "\xff", "本")
	f.Fuzz(func(t *testing.T, first, middle, last string) {
		oracle := fnv.New64a()
		_, _ = oracle.Write([]byte(first + middle + last))
		got := core.FNV1a64TextParts(first, middle, last)
		if got != oracle.Sum64() {
			t.Fatalf("parts fingerprint = %x, want native concatenation %x", got, oracle.Sum64())
		}
		if got != core.FNV1a64TextParts(first+middle, last) || got != core.FNV1a64TextParts(first, middle+last) {
			t.Fatal("part boundary changed exact input bytes")
		}
	})
}
