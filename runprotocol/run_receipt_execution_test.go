package runprotocol

import (
	"errors"
	"github.com/deliri/primitive/v2026/core"
	"testing"
)

func TestCompactReceiptClosedDomains(t *testing.T) {
	t.Parallel()
	for n := 0; n < 256; n++ {
		for _, tc := range []struct {
			name     string
			limit    int
			validate func() error
		}{
			{"workload kind", 3, func() error { return RunKind(n).Validate() }},
			{"source kind", 2, func() error { return RunSourceKind(n).Validate() }},
			{"scope kind", 4, func() error { return RunScopeKind(n).Validate() }},
			{"fact kind", 3, func() error { return RunFactKind(n).Validate() }},
			{"result kind", 4, func() error { return RunResultKind(n).Validate() }},
		} {
			err := tc.validate()
			want := n > 0 && n <= tc.limit
			if (err == nil) != want || !want && !errors.Is(err, core.ErrRunProtocolContract) {
				t.Fatalf("%s value%d = %v, want admitted%t", tc.name, n, err, want)
			}
		}
	}
}
func TestCompactReceiptScalarBoundaries(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, value string
		admit       bool
	}{
		{"observed zero seed", "0", true}, {"maximum signed seed", "9223372036854775807", true}, {"minimum signed seed", "-9223372036854775808", true},
		{"unobserved empty seed", "", false}, {"future positive overflow", "9223372036854775808", false}, {"future negative overflow", "-9223372036854775809", false},
		{"ambiguous positive sign", "+1", false}, {"ambiguous negative zero", "-0", false}, {"noncanonical leading zero", "01", false}, {"runtime choice is not actual seed", "on", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			v, err := ParseRunShuffleSeed(tc.value)
			if (err == nil) != tc.admit || !tc.admit && (!errors.Is(err, core.ErrRunProtocolContract) || v != (RunShuffleSeed{})) {
				t.Fatalf("ParseRunShuffleSeed(%q) = (%+v,%v), want admitted%t", tc.value, v, err, tc.admit)
			}
			if tc.admit {
				b, err := v.MarshalJSON()
				if err != nil {
					t.Fatal(err)
				}
				var round RunShuffleSeed
				if err := round.UnmarshalJSON(b); err != nil || round != v {
					t.Fatalf("native seed roundtrip = (%+v,%v), want %+v", round, err, v)
				}
			}
		})
	}
	for _, tc := range []struct {
		name, value string
		admit       bool
	}{
		{"constructed empty argv remains an argument", "", true}, {"exact flag boundaries", "-run=^TestNative/a b$", true}, {"native unicode", "界é", true},
		{"native NUL refuses", "a\x00b", false}, {"invalid JSON UTF8 cannot be invented replacement", string([]byte{255}), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			v, err := NewRunArgument(tc.value)
			if (err == nil) != tc.admit || !tc.admit && (!errors.Is(err, core.ErrRunProtocolContract) || v != (RunArgument{})) {
				t.Fatalf("NewRunArgument = (%q,%v), want admitted%t", v.String(), err, tc.admit)
			}
			if tc.admit {
				b, err := v.MarshalJSON()
				if err != nil {
					t.Fatal(err)
				}
				var round RunArgument
				if err := round.UnmarshalJSON(b); err != nil || round != v {
					t.Fatalf("native argv roundtrip = (%q,%v), want %q", round.String(), err, v.String())
				}
			}
		})
	}
}
