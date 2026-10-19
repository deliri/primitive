package runnercontrol

import (
	"math"
	"strconv"
)

// Float64 rounding requires only a finite significant prefix. Go's strconv
// decimal implementation uses 800 digits; discarded nonzero digits are sticky.
// This window limits storage, not input length or exponent spelling length.
type goJSONFloat struct {
	digits           [800]byte
	length           int
	scale            int64
	exponent         uint64
	fraction         bool
	nonzero          bool
	negative         bool
	sticky           bool
	exponentPart     bool
	exponentNegative bool
	exponentOverflow bool
}

func (goJSONFloat) runnerControlInternalFlow() {}

func (n *goJSONFloat) consume(value byte) {
	switch value {
	case '.':
		n.fraction = true
		return
	case 'e', 'E':
		n.exponentPart = true
		return
	case '-':
		if n.exponentPart {
			n.exponentNegative = true
		} else {
			n.negative = true
		}
		return
	case '+':
		return
	}
	if value < '0' || value > '9' {
		return
	}
	n.consumeDigit(value)
}

func (n *goJSONFloat) consumeDigit(value byte) {
	if n.exponentPart {
		digit := uint64(value - '0')
		if n.exponent > (math.MaxInt64-digit)/10 {
			n.exponentOverflow = true
			return
		}
		n.exponent = n.exponent*10 + digit
		return
	}
	if !n.nonzero && value == '0' {
		if n.fraction {
			n.scale--
		}
		return
	}
	n.nonzero = true
	if !n.fraction {
		n.scale++
	}
	if n.length < len(n.digits) {
		n.digits[n.length] = value
		n.length++
	} else if value != '0' {
		n.sticky = true
	}
}

func (n *goJSONFloat) value() (float64, error) {
	zero := 0.0
	if n.negative {
		zero = math.Copysign(0, -1)
	}
	if !n.nonzero {
		return zero, nil
	}
	if n.exponentOverflow {
		if n.exponentNegative {
			return zero, nil
		}
		return 0, goJSONFailure()
	}
	if n.exponent > math.MaxInt64 {
		return 0, goJSONFailure()
	}
	exponent := int64(n.exponent)
	if n.exponentNegative {
		exponent = -exponent
	}
	if exponent > 0 && n.scale > math.MaxInt64-exponent {
		return 0, goJSONFailure()
	}
	if exponent < 0 && n.scale < math.MinInt64-exponent {
		return zero, nil
	}
	exponent += n.scale
	return n.decimalValue(exponent)
}

func (n *goJSONFloat) decimalValue(exponent int64) (float64, error) {
	// A sticky digit keeps an exact halfway prefix from becoming an exact tie.
	decimal := "0." + string(n.digits[:n.length])
	if n.sticky {
		decimal += "1"
	}
	if n.negative {
		decimal = "-" + decimal
	}
	value, err := strconv.ParseFloat(decimal+"e"+strconv.FormatInt(exponent, 10), 64)
	if err != nil {
		return 0, goJSONFailure()
	}
	return value, nil
}
