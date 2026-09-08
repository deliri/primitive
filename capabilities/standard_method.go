package capabilities

import "slices"

// standardMethodRule binds the declaring receiver type, never a variable name
// or an imported package alias. A zero effect explicitly marks pure methods.
type standardMethodRule struct {
	importPath  string
	receiver    string
	selectors   []string
	effect      Effect
	disposition StandardSymbolDisposition
}

func resolveStandardMethod(symbol StandardSymbol) (StandardSymbolFact, error) {
	return resolveMethodRules(symbol, standardMethodRules(symbol.ImportPath.String()))
}

func resolveMethodRules(symbol StandardSymbol, rules []standardMethodRule) (StandardSymbolFact, error) {
	fact := StandardSymbolFact{Symbol: symbol, Disposition: StandardSymbolUnresolved}
	for _, rule := range rules {
		if rule.importPath != symbol.ImportPath.String() || rule.receiver != symbol.Receiver.String() || !slices.Contains(rule.selectors, symbol.Selector.String()) {
			continue
		}
		candidate := Classification{Effect: rule.effect, Disposition: rule.disposition}
		if err := mergeClassification(&fact.Classification, candidate); err != nil {
			return StandardSymbolFact{}, err
		}
	}
	return fact, fact.Validate()
}
