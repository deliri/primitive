package capabilities

// Text operations classify by reviewed selector, never by package membership.
// Func and Map operations execute caller code and require compiler context.
func standardTextFunctionRules(importPath string) []standardSymbolRule {
	switch importPath {
	case "strings":
		return []standardSymbolRule{{importPath: importPath,
			pureSelectors:       []string{"Clone", "Compare", "Contains", "ContainsAny", "ContainsRune", "Count", "Cut", "CutPrefix", "CutSuffix", "CutLast", "EqualFold", "Fields", "FieldsSeq", "HasPrefix", "HasSuffix", "Index", "IndexAny", "IndexByte", "IndexRune", "Join", "LastIndex", "LastIndexAny", "LastIndexByte", "Lines", "NewReader", "NewReplacer", "Repeat", "Replace", "ReplaceAll", "Split", "SplitAfter", "SplitAfterN", "SplitAfterSeq", "SplitN", "SplitSeq", "Title", "ToLower", "ToLowerSpecial", "ToTitle", "ToTitleSpecial", "ToUpper", "ToUpperSpecial", "ToValidUTF8", "Trim", "TrimLeft", "TrimPrefix", "TrimRight", "TrimSpace", "TrimSuffix"},
			contextualSelectors: textCallbackSelectors(),
		}}
	case "bytes":
		return []standardSymbolRule{{importPath: importPath,
			pureSelectors:       []string{"Clone", "Compare", "Contains", "ContainsAny", "ContainsRune", "Count", "Cut", "CutPrefix", "CutSuffix", "CutLast", "Equal", "EqualFold", "Fields", "FieldsSeq", "HasPrefix", "HasSuffix", "Index", "IndexAny", "IndexByte", "IndexRune", "Join", "LastIndex", "LastIndexAny", "LastIndexByte", "Lines", "NewBuffer", "NewBufferString", "NewReader", "Repeat", "Replace", "ReplaceAll", "Runes", "Split", "SplitAfter", "SplitAfterN", "SplitAfterSeq", "SplitN", "SplitSeq", "Title", "ToLower", "ToLowerSpecial", "ToTitle", "ToTitleSpecial", "ToUpper", "ToUpperSpecial", "ToValidUTF8", "Trim", "TrimLeft", "TrimPrefix", "TrimRight", "TrimSpace", "TrimSuffix"},
			contextualSelectors: textCallbackSelectors(),
		}}
	case "strconv":
		return []standardSymbolRule{{importPath: importPath,
			pureSelectors: []string{"AppendBool", "AppendFloat", "AppendInt", "AppendQuote", "AppendQuoteRune", "AppendQuoteRuneToASCII", "AppendQuoteRuneToGraphic", "AppendQuoteToASCII", "AppendQuoteToGraphic", "AppendUint", "Atoi", "CanBackquote", "FormatBool", "FormatComplex", "FormatFloat", "FormatInt", "FormatUint", "IsGraphic", "IsPrint", "Itoa", "ParseBool", "ParseComplex", "ParseFloat", "ParseInt", "ParseUint", "Quote", "QuotedPrefix", "QuoteRune", "QuoteRuneToASCII", "QuoteRuneToGraphic", "QuoteToASCII", "QuoteToGraphic", "Unquote", "UnquoteChar"},
		}}
	}
	return nil
}

func textCallbackSelectors() []string {
	return []string{"ContainsFunc", "FieldsFunc", "FieldsFuncSeq", "IndexFunc", "LastIndexFunc", "Map", "TrimFunc", "TrimLeftFunc", "TrimRightFunc"}
}
