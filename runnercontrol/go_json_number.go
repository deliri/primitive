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
		}
		return
	case '+':
		return
	}
	if value < '0' || value > '9' {
		return
	}
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

func (n *goJSONFloat) validate() error {
	if !n.nonzero {
		return nil
	}
	if n.exponentOverflow {
		if n.exponentNegative {
			return nil
		}
		return goJSONFailure()
	}
	exponent := int64(n.exponent) // bounded during accumulation.
	if n.exponentNegative {
		exponent = -exponent
	}
	if exponent > 0 && n.scale > math.MaxInt64-exponent {
		return goJSONFailure()
	}
	if exponent < 0 && n.scale < math.MinInt64-exponent {
		return nil
	}
	exponent += n.scale
	// A sticky digit keeps an exact halfway prefix from becoming an exact tie.
	decimal := "0." + string(n.digits[:n.length])
	if n.sticky {
		decimal += "1"
	}
	_, err := strconv.ParseFloat(decimal+"e"+strconv.FormatInt(exponent, 10), 64)
	if err != nil {
		return goJSONFailure()
	}
	return nil
}
