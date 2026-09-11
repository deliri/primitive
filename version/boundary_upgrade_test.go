package version_test

import (
	"bytes"
	json "encoding/json/v2"
	"errors"
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/version"
)

// The oracle uses independently parsed unsigned coordinates and canonical
// decimal formatting; it does not call a Primitive validator.
func admittedTag(text string) bool {
	if !strings.HasPrefix(text, "v") {
		return false
	}
	parts := strings.SplitN(text[1:], ".", 4)
	if len(parts) != 3 {
		return false
	}
	for _, part := range parts {
		value, err := strconv.ParseUint(part, 10, 32)
		if err != nil || strconv.FormatUint(value, 10) != part {
			return false
		}
	}
	return true
}

func TestTagJSONBoundaryLayerTriad(t *testing.T) {
	t.Parallel()
	want, err := version.ParseTag("v4294967295.4294967295.4294967295")
	if err != nil {
		t.Fatal(err)
	}
	wire, err := want.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	var escaped strings.Builder
	escaped.WriteByte('"')
	for _, value := range []byte(want.String()) {
		escaped.WriteString(`\u00`)
		escaped.WriteString(strconv.FormatUint(uint64(value), 16))
	}
	escaped.WriteByte('"')
	for _, tc := range []struct {
		name    string
		data    []byte
		want    version.Tag
		wantErr bool
	}{
		{name: "maximum nominal coordinates", data: wire, want: want},
		{name: "fully escaped maximum coordinates", data: []byte(escaped.String()), want: want},
		{name: "whitespace extent does not limit the document", data: append(append(bytes.Repeat([]byte(" \t\r\n"), 262144), wire...), bytes.Repeat([]byte(" \t\r\n"), 262144)...), want: want},
		{name: "absent input preserves existing receiver", wantErr: true},
		{name: "null cannot manufacture tag", data: []byte("null"), wantErr: true},
		{name: "oversized decoded token", data: []byte(`"v` + strings.Repeat("1", 1048576) + `"`), wantErr: true},
		{name: "trailing JSON value", data: append(bytes.Clone(wire), []byte(" null")...), wantErr: true},
		{name: "unicode whitespace is not JSON whitespace", data: append([]byte("\u00a0"), wire...), wantErr: true},
		{name: "wrong JSON type", data: []byte("123"), wantErr: true},
		{name: "unpaired surrogate", data: []byte(`"\ud800"`), wantErr: true},
		{name: "uint32 overflow", data: []byte(`"v4294967296.0.0"`), wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := want
			err := got.UnmarshalJSON(tc.data)
			if tc.wantErr {
				if !errors.Is(err, core.ErrJSONContract) || !errors.Is(err, core.ErrReleaseContract) || got != want {
					t.Fatalf("UnmarshalJSON refusal = %v/%v, want preserved typed refusal", got, err)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Fatalf("UnmarshalJSON = %v/%v, want %v/nil", got, err, tc.want)
			}
		})
	}
}

func TestTagNilAndZeroReceiverContracts(t *testing.T) {
	t.Parallel()
	var nilTag *version.Tag
	if err := nilTag.UnmarshalText([]byte("v0.0.0")); !errors.Is(err, core.ErrReleaseContract) {
		t.Fatalf("nil.UnmarshalText = %v, want release refusal", err)
	}
	if err := nilTag.UnmarshalJSON([]byte(`"v0.0.0"`)); !errors.Is(err, core.ErrReleaseContract) || !errors.Is(err, core.ErrJSONContract) {
		t.Fatalf("nil.UnmarshalJSON = %v, want release and JSON refusal", err)
	}
	var zero version.Tag
	if data, err := zero.MarshalText(); data != nil || !errors.Is(err, core.ErrReleaseContract) {
		t.Fatalf("zero.MarshalText = %q/%v, want nil/release refusal", data, err)
	}
	if data, err := zero.MarshalJSON(); data != nil || !errors.Is(err, core.ErrReleaseContract) || !errors.Is(err, core.ErrJSONContract) {
		t.Fatalf("zero.MarshalJSON = %q/%v, want nil/typed refusal", data, err)
	}
	if zero.String() != "" || zero.Release() != (version.Release{}) {
		t.Fatalf("zero tag = %q/%v, want empty/zero", zero.String(), zero.Release())
	}
}

func FuzzTagJSONSemanticClosure(f *testing.F) {
	seed := releaseFromCoordinates(f, 2026, 1, 3).Tag()
	canonical, err := seed.MarshalJSON()
	if err != nil {
		f.Fatal(err)
	}
	f.Add(canonical)
	f.Add([]byte("null"))
	f.Add([]byte(`"v4294967296.0.0"`))
	f.Add([]byte(`"\u00760.0.0"`))
	f.Fuzz(func(t *testing.T, data []byte) {
		var text string
		token := bytes.Trim(data, " \t\r\n")
		// At most 33 ASCII tag bytes, each escaped as six JSON bytes, plus quotes.
		// Larger tokens cannot represent this domain; keep the independent oracle bounded too.
		nativeErr := error(core.ErrJSONContract)
		if len(token) <= 200 {
			nativeErr = json.Unmarshal(token, &text)
		}
		wantAccepted := nativeErr == nil && admittedTag(text)
		got := seed
		err := got.UnmarshalJSON(data)
		if !wantAccepted {
			if !errors.Is(err, core.ErrJSONContract) || !errors.Is(err, core.ErrReleaseContract) || got != seed {
				t.Fatalf("JSON refusal = %v/%v, want preserved typed refusal", got, err)
			}
			return
		}
		if err != nil || got.Validate() != nil || got.String() != text {
			t.Fatalf("JSON admission = %v/%v, want %q", got, err, text)
		}
		first, err := got.MarshalJSON()
		if err != nil {
			t.Fatal(err)
		}
		var round version.Tag
		if err := round.UnmarshalJSON(first); err != nil || round != got {
			t.Fatalf("JSON round trip = %v/%v, want %v/nil", round, err, got)
		}
		second, err := round.MarshalJSON()
		if err != nil || !bytes.Equal(first, second) {
			t.Fatalf("canonical JSON = %q/%v, want %q/nil", second, err, first)
		}
	})
}

func TestReleaseCompareCoordinatePrecedence(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		a, b [3]uint32
		want core.Comparison
	}{
		{name: "major dominates maximal lower coordinates", a: [3]uint32{1, math.MaxUint32, math.MaxUint32}, b: [3]uint32{2, 0, 0}, want: core.ComparisonLess},
		{name: "minor dominates maximal patch", a: [3]uint32{2, 1, math.MaxUint32}, b: [3]uint32{2, 2, 0}, want: core.ComparisonLess},
		{name: "patch orders adjacent extremes", a: [3]uint32{2, 2, math.MaxUint32 - 1}, b: [3]uint32{2, 2, math.MaxUint32}, want: core.ComparisonLess},
		{name: "equal coordinates preserve equality", a: [3]uint32{2, 2, 2}, b: [3]uint32{2, 2, 2}, want: core.ComparisonEqual},
		{name: "greater major cannot be outvoted", a: [3]uint32{math.MaxUint32, 0, 0}, b: [3]uint32{1, math.MaxUint32, math.MaxUint32}, want: core.ComparisonGreater},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a := releaseFromCoordinates(t, tc.a[0], tc.a[1], tc.a[2])
			b := releaseFromCoordinates(t, tc.b[0], tc.b[1], tc.b[2])
			got, err := a.Compare(b)
			if err != nil || got != tc.want {
				t.Fatalf("Compare = %v/%v, want %v/nil", got, err, tc.want)
			}
		})
	}
	zero := version.Release{}
	valid := releaseFromCoordinates(t, 2026, 1, 3)
	for _, pair := range [][2]version.Release{{zero, valid}, {valid, zero}, {zero, zero}} {
		got, err := pair[0].Compare(pair[1])
		if got != core.ComparisonUnknown || !errors.Is(err, core.ErrReleaseContract) {
			t.Fatalf("invalid Compare = %v/%v, want unknown/release refusal", got, err)
		}
	}
}

func BenchmarkTagJSONOversizedRefusal(b *testing.B) {
	data := []byte(`"v` + strings.Repeat("1", 1048576) + `"`)
	seed := releaseFromCoordinates(b, 2026, 1, 3).Tag()
	b.ReportAllocs()
	for b.Loop() {
		got := seed
		err := got.UnmarshalJSON(data)
		if !errors.Is(err, core.ErrReleaseContract) || got != seed {
			b.Fatalf("oversized JSON = %v/%v, want preserved/refusal", got, err)
		}
	}
}
