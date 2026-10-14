package currency

// ParseProviderCode accepts exactly three ASCII letters, case-insensitively,
// then admits them through the same closed domain as ParseCode. Whitespace,
// non-ASCII case folding and unknown currencies remain refused.
func ParseProviderCode(token string) (Code, error) {
	if len(token) != 3 {
		return CodeUnknown, contractError(currencyTokenNotAdmittedReason)
	}
	var canonical [3]byte
	for index := range canonical {
		value := token[index]
		if value >= 'a' && value <= 'z' {
			value -= 'a' - 'A'
		}
		if value < 'A' || value > 'Z' {
			return CodeUnknown, contractError(currencyTokenNotAdmittedReason)
		}
		canonical[index] = value
	}
	return ParseCode(string(canonical[:]))
}

// ProviderCode returns the lowercase ASCII currency token used by Stripe.
// Source: https://docs.stripe.com/currencies
func (c Code) ProviderCode() (string, error) {
	if err := c.Validate(); err != nil {
		return "", err
	}
	canonical := c.String()
	var output [3]byte
	for index := range output {
		output[index] = canonical[index] + ('a' - 'A')
	}
	return string(output[:]), nil
}
