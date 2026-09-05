package capabilities

import (
	"bytes"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gomodule"
)

// Exhausts the complete disposition byte domain against every effect owner.
func TestClassificationLayerTriad(t *testing.T) {
	t.Parallel()
	for raw := 0; raw <= 255; raw++ {
		for owner := EffectUnknown; int(owner) <= IdentityCount+1; owner++ {
			fact := Classification{Disposition: StandardSymbolDisposition(raw), Effect: owner}
			wantValid := (raw == int(StandardSymbolEffect) && owner.Validate() == nil) ||
				(owner == EffectUnknown && (raw == int(StandardSymbolPure) || raw == int(StandardSymbolContextual) || raw == int(StandardSymbolUnresolved)))
			err := fact.Validate()
			if (err == nil) != wantValid {
				t.Fatalf("classification %v/%v validation = %v, want valid:%t", raw, owner, err, wantValid)
			}
			if !wantValid {
				if !errors.Is(err, core.ErrCapabilitiesContract) {
					t.Fatalf("classification rejection = %v, want %v", err, core.ErrCapabilitiesContract)
				}
				continue
			}
			data, err := fact.MarshalJSON()
			if err != nil {
				t.Fatal(err)
			}
			var got Classification
			if err := got.UnmarshalJSON(data); err != nil || !got.Equal(fact) {
				t.Fatalf("classification round trip = (%+v,%v), want %+v", got, err, fact)
			}
		}
	}
}

func TestCatalogConflictsAndNeutralityLayerTriad(t *testing.T) {
	t.Parallel()
	path, err := gomodule.ParseImportPath("os")
	if err != nil {
		t.Fatal(err)
	}
	selector, err := ParseSymbolName("ReadFile")
	if err != nil {
		t.Fatal(err)
	}
	symbol := StandardSymbol{ImportPath: path, Selector: selector}
	effect := standardSymbolRule{importPath: path.String(), effect: EffectFilesystem, effectSelectors: []string{selector.String()}}
	pure := standardSymbolRule{importPath: path.String(), pureSelectors: []string{selector.String()}}
	cases := []struct {
		name    string
		rules   []standardSymbolRule
		want    StandardSymbolDisposition
		wantErr error
	}{
		{"one admitted effect", []standardSymbolRule{effect}, StandardSymbolEffect, nil},
		{"duplicate identical fact is idempotent", []standardSymbolRule{effect, effect}, StandardSymbolEffect, nil},
		{"same symbol conflicting facts refuse", []standardSymbolRule{effect, pure}, StandardSymbolUnknown, core.ErrCapabilitiesContract},
		{"reversed contradiction still refuses", []standardSymbolRule{pure, effect}, StandardSymbolUnknown, core.ErrCapabilitiesContract},
		{"absence retains unresolved", nil, StandardSymbolUnresolved, nil},
		{"intra-rule contradictory dispositions refuse", []standardSymbolRule{{importPath: path.String(), effect: EffectFilesystem, effectSelectors: effect.effectSelectors, pureSelectors: pure.pureSelectors}}, StandardSymbolUnknown, core.ErrCapabilitiesContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := resolveFunctionRules(symbol, tc.rules)
			if !errors.Is(err, tc.wantErr) || got.Disposition != tc.want {
				t.Fatalf("catalog = (%+v,%v), want disposition:%v error:%v", got, err, tc.want, tc.wantErr)
			}
			if err != nil && (got.Effect != EffectUnknown || len(got.Secondary) != 0) {
				t.Fatalf("refused effects = %+v, want no escaped ownership", got)
			}
		})
	}
}

func FuzzClassificationJSONSemanticClosure(f *testing.F) {
	for disposition := StandardSymbolPure; disposition <= StandardSymbolUnresolved; disposition++ {
		fact := Classification{Disposition: disposition}
		if disposition == StandardSymbolEffect {
			fact.Effect = EffectTransport
			fact.Secondary = []Effect{EffectFilesystem}
		}
		data, err := fact.MarshalJSON()
		if err != nil {
			f.Fatal(err)
		}
		f.Add(data)
	}
	f.Add([]byte{})
	f.Add([]byte(`null`))
	f.Add([]byte(`{"disposition":"pure","disposition":"effect"}`))
	f.Add(bytes.Repeat([]byte(" "), ClassificationJSONMaximumBytes+1))
	f.Fuzz(func(t *testing.T, data []byte) {
		baseline := Classification{Disposition: StandardSymbolPure}
		got := baseline
		err := got.UnmarshalJSON(data)
		if err != nil {
			if !errors.Is(err, core.ErrCapabilitiesContract) || !got.Equal(baseline) {
				t.Fatalf("rejected classification = (%+v,%v), want unchanged and %v", got, err, core.ErrCapabilitiesContract)
			}
			return
		}
		if err := got.Validate(); err != nil {
			t.Fatal(err)
		}
		encoded, err := got.MarshalJSON()
		if err != nil || len(encoded) > ClassificationJSONMaximumBytes {
			t.Fatalf("accepted encoding = (%d,%v), want bounded", len(encoded), err)
		}
		var second Classification
		if err := second.UnmarshalJSON(encoded); err != nil || !second.Equal(got) {
			t.Fatalf("canonical decode = (%+v,%v), want %+v", second, err, got)
		}
		again, err := second.MarshalJSON()
		if err != nil || !bytes.Equal(encoded, again) {
			t.Fatalf("second encoding = (%q,%v), want %q", again, err, encoded)
		}
	})
}

func FuzzStandardSymbolDispositionJSONSemanticClosure(f *testing.F) {
	for d := StandardSymbolPure; d <= StandardSymbolUnresolved; d++ {
		data, err := d.MarshalJSON()
		if err != nil {
			f.Fatal(err)
		}
		f.Add(data)
	}
	f.Add([]byte{})
	f.Add([]byte(`"future"`))
	f.Fuzz(func(t *testing.T, data []byte) {
		got := StandardSymbolPure
		err := got.UnmarshalJSON(data)
		if err != nil {
			if !errors.Is(err, core.ErrCapabilitiesContract) || got != StandardSymbolPure {
				t.Fatalf("decode = (%v,%v), want unchanged typed refusal", got, err)
			}
			return
		}
		if err := got.Validate(); err != nil {
			t.Fatal(err)
		}
		encoded, err := got.MarshalJSON()
		if err != nil {
			t.Fatal(err)
		}
		var second StandardSymbolDisposition
		if err := second.UnmarshalJSON(encoded); err != nil || second != got {
			t.Fatalf("round trip = (%v,%v), want %v", second, err, got)
		}
	})
}
