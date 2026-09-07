package capabilities

import (
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gomodule"
)

type handoffClass uint8

const (
	handoffContradiction handoffClass = iota
	handoffRefusal
	handoffNeutral
	handoffBoundary
)

// Exhaust the Cartesian product of every disposition/effect outcome emitted
// by a single rule, including every distinct primary/secondary pair. This is
// the actual rule.resolve -> mergeClassification -> resolveFunctionRules seam.
func TestRuleProducerClassifierHandoff(t *testing.T) {
	t.Parallel()
	symbol := handoffSymbol(t)
	states := []Classification{{Disposition: StandardSymbolUnresolved}, {Disposition: StandardSymbolPure}, {Disposition: StandardSymbolContextual}}
	for effect := EffectFilesystem; effect < effectLimit; effect++ {
		states = append(states, Classification{Disposition: StandardSymbolEffect, Effect: effect})
	}
	for primary := EffectFilesystem; primary < effectLimit; primary++ {
		for secondary := EffectFilesystem; secondary < effectLimit; secondary++ {
			if primary != secondary {
				states = append(states, Classification{Disposition: StandardSymbolEffect, Effect: primary, Secondary: []Effect{secondary}})
			}
		}
	}
	for leftIndex, left := range states {
		for rightIndex, right := range states {
			class := handoffBoundary
			want := left
			var wantErr error
			switch {
			case left.Disposition == StandardSymbolUnresolved && right.Disposition == StandardSymbolUnresolved:
				class = handoffNeutral
			case left.Disposition == StandardSymbolUnresolved:
				want = right
				class = handoffNeutral
			case right.Disposition == StandardSymbolUnresolved:
				class = handoffNeutral
			case !left.Equal(right):
				class = handoffContradiction
				want = Classification{}
				wantErr = core.ErrCapabilitiesContract
			}
			t.Run(fmt.Sprintf("class_%d_left_%d_right_%d", class, leftIndex, rightIndex), func(t *testing.T) {
				t.Parallel()
				first := handoffRule(symbol, left)
				second := handoffRule(symbol, right)
				for _, source := range []struct {
					rule standardSymbolRule
					want Classification
				}{{first, left}, {second, right}} {
					fact, err := source.rule.resolve(symbol, symbol.Selector.String())
					if err != nil || fact.Symbol != symbol || !fact.Classification.Equal(source.want) {
						t.Fatalf("producer = (%+v,%v), want retained %+v", fact, err, source.want)
					}
				}
				variants := [][]standardSymbolRule{{first, second}, {second, first}, {first, second, first}, {second, first, second}}
				for _, rules := range variants {
					got, err := resolveFunctionRules(symbol, rules)
					if !errors.Is(err, wantErr) || !got.Classification.Equal(want) {
						t.Fatalf("handoff class %d = (%+v,%v), want %+v / %v", class, got, err, want, wantErr)
					}
					if err != nil {
						if got.Symbol != (StandardSymbol{}) {
							t.Fatalf("contradiction leaked symbol %+v", got.Symbol)
						}
						continue
					}
					if got.Symbol != symbol {
						t.Fatalf("handoff changed subject %+v", got.Symbol)
					}
					// Ownership must equal an actual producer's complete ownership set.
					if !got.Classification.Equal(left) && !got.Classification.Equal(right) {
						t.Fatalf("invented ownership %+v", got)
					}
				}
			})
		}
	}
	// Exhaust every inadmissible effect byte through producer and classifier.
	for raw := range 256 {
		if raw >= int(EffectFilesystem) && raw < int(effectLimit) {
			continue
		}
		t.Run(fmt.Sprintf("class_%d_invalid_effect_%d", handoffRefusal, raw), func(t *testing.T) {
			t.Parallel()
			rule := handoffRule(symbol, Classification{Disposition: StandardSymbolEffect, Effect: Effect(raw)})
			fact, sourceErr := rule.resolve(symbol, symbol.Selector.String())
			if !errors.Is(sourceErr, core.ErrCapabilitiesContract) {
				t.Fatalf("producer admitted %+v: %v", fact, sourceErr)
			}
			got, err := resolveFunctionRules(symbol, []standardSymbolRule{rule})
			if !errors.Is(err, core.ErrCapabilitiesContract) || got.Symbol != (StandardSymbol{}) || !got.Classification.Equal(Classification{}) {
				t.Fatalf("refusal lost: (%+v,%v)", got, err)
			}
		})
	}
}

func handoffSymbol(t testing.TB) StandardSymbol {
	t.Helper()
	path, err := gomodule.ParseImportPath("os")
	if err != nil {
		t.Fatal(err)
	}
	selector, err := ParseSymbolName(symbolReadFile)
	if err != nil {
		t.Fatal(err)
	}
	return StandardSymbol{ImportPath: path, Selector: selector}
}
func handoffRule(symbol StandardSymbol, value Classification) standardSymbolRule {
	rule := standardSymbolRule{importPath: symbol.ImportPath.String()}
	selector := []string{symbol.Selector.String()}
	switch value.Disposition {
	case StandardSymbolPure:
		rule.pureSelectors = selector
	case StandardSymbolContextual:
		rule.contextualSelectors = selector
	case StandardSymbolEffect:
		rule.effectSelectors = selector
		rule.effect = value.Effect
		if len(value.Secondary) != 0 {
			rule.secondary = value.Secondary[0]
			rule.secondarySelectors = slices.Clone(selector)
		}
	}
	return rule
}
