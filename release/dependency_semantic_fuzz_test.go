package release

import (
	"bytes"
	"encoding/base64"
	json "encoding/json/v2"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func FuzzBuildDependenciesExactFacts(f *testing.F) {
	var baseline BuildDependencies
	var baselineBytes []byte
	for _, count := range []int{0, 1, BuildDependencyMaximumCount} {
		seed, err := newBuildDependencies(mustModulePath(f, testMainModule), CurrentGoToolchain(), numberedModules(f, count))
		if err != nil {
			f.Fatalf("newBuildDependencies(seed) error = %v, want nil", err)
		}
		if err := seed.Validate(); err != nil {
			f.Fatalf("BuildDependencies.Validate(seed) error = %v, want nil", err)
		}
		encoded, err := seed.MarshalJSON()
		if err != nil {
			f.Fatalf("BuildDependencies.MarshalJSON(seed) error = %v, want nil", err)
		}
		f.Add(encoded)
		if count == 1 {
			baseline = seed
			baselineBytes = bytes.Clone(encoded)
		}
	}
	f.Add([]byte(`null`))
	f.Add([]byte(`{}`))
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, data []byte) {
		got := baseline
		gotErr := got.UnmarshalJSON(data)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrReleaseContract) || !errors.Is(gotErr, core.ErrJSONContract) || got != baseline {
				t.Fatalf("BuildDependencies.UnmarshalJSON(refused) = (%v, %v), want preserved receiver and typed refusal", got, gotErr)
			}
			preserved, err := got.MarshalJSON()
			if err != nil || !bytes.Equal(preserved, baselineBytes) {
				t.Fatalf("refused dependency custody = (%q, %v), want original facts %q", preserved, err, baselineBytes)
			}
			return
		}
		if len(data) > dependencyDocumentExtentMaximum {
			t.Fatalf("admitted document extent = %d, want <= %d", len(data), dependencyDocumentExtentMaximum)
		}
		// Go independently extracts the input facts. Production round trips alone
		// cannot catch a decoder that silently rewrites one checksum or module.
		var wire buildDependenciesWire
		if err := json.Unmarshal(data, &wire); err != nil {
			t.Fatalf("Go Unmarshal(admitted input) error = %v, want nil", err)
		}
		if err := got.Validate(); err != nil || got.MainModule().String() != wire.MainModule || got.Count() != len(wire.Modules) {
			t.Fatalf("admitted dependency document = (%v, %v), want exact main and module count", got, err)
		}
		toolchain, err := got.GoToolchain().Version()
		if err != nil || toolchain != wire.GoToolchain {
			t.Fatalf("admitted toolchain = (%q, %v), want %q", toolchain, err, wire.GoToolchain)
		}
		for index, want := range wire.Modules {
			module, ok := got.At(index)
			if !ok || module.Path().String() != want.Path || module.Version().String() != want.Version || module.Sum().String() != want.Sum {
				t.Fatalf("admitted module %d = (%v, %t), want exact input facts %v", index, module, ok, want)
			}
			if len(want.Sum) != goModuleSumMaximumBytes || !bytes.HasPrefix([]byte(want.Sum), []byte(goModuleSumPrefix)) {
				t.Fatalf("admitted checksum = %q, want exact algorithm and extent", want.Sum)
			}
			decoded, err := base64.StdEncoding.DecodeString(want.Sum[len(goModuleSumPrefix):])
			if err != nil || len(decoded) != core.SHA256DigestBytes || base64.StdEncoding.EncodeToString(decoded) != want.Sum[len(goModuleSumPrefix):] || !bytes.Equal(decoded, module.sum.digest[:]) {
				t.Fatalf("admitted checksum %q = (%x, %v), want exact canonical Go digest", want.Sum, decoded, err)
			}
		}
		canonical, err := got.MarshalJSON()
		if err != nil || len(canonical) > dependencyDocumentExtentMaximum {
			t.Fatalf("BuildDependencies.MarshalJSON() = (%d bytes, %v), want bounded output and nil", len(canonical), err)
		}
		var roundTrip BuildDependencies
		if err := roundTrip.UnmarshalJSON(canonical); err != nil {
			t.Fatalf("BuildDependencies.UnmarshalJSON(canonical) error = %v, want nil", err)
		}
		second, err := roundTrip.MarshalJSON()
		if err != nil || !bytes.Equal(second, canonical) {
			t.Fatalf("second canonical document = (%q, %v), want %q", second, err, canonical)
		}
	})
}
