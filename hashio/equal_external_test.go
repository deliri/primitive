package hashio_test

import (
	"bytes"
	"slices"
	"strconv"
	"testing"

	"github.com/deliri/primitive/v2026/hashio"
)

func TestEqualBytesPreservesExactNativeEquality(t *testing.T) {
	t.Parallel()
	for _, width := range []int{0, 1, 63, 64, 65, 127, 128, 129, 4097} {
		t.Run(strconv.Itoa(width), func(t *testing.T) {
			t.Parallel()
			first := bytes.Repeat([]byte{0xff}, width)
			copy := slices.Clone(first)
			if !hashio.EqualBytes(first, copy) || !hashio.EqualBytes(copy, first) {
				t.Fatal("identical byte strings refused")
			}
			for position := range width {
				copy[position] ^= 1
				if hashio.EqualBytes(first, copy) || hashio.EqualBytes(copy, first) {
					t.Fatalf("different byte at %d was accepted", position)
				}
				copy[position] ^= 1
			}
			if hashio.EqualBytes(first, append(slices.Clone(copy), 0)) || hashio.EqualBytes(append(slices.Clone(copy), 0), first) {
				t.Fatal("different extents were accepted")
			}
		})
	}
	if !hashio.EqualBytes(nil, []byte{}) {
		t.Fatal("native empty equality was refused")
	}
}

func FuzzEqualBytesExactNativeEquality(f *testing.F) {
	f.Add([]byte{}, []byte{})
	f.Add([]byte{0, 255}, []byte{0, 255})
	f.Add([]byte{0, 255}, []byte{0, 254})
	f.Fuzz(func(t *testing.T, first, second []byte) {
		got := hashio.EqualBytes(first, second)
		if want := slices.Equal(first, second); got != want {
			t.Fatalf("equality = %t, want exact native bytes %t", got, want)
		}
		if got != hashio.EqualBytes(second, first) {
			t.Fatal("comparison lost symmetry")
		}
	})
}
