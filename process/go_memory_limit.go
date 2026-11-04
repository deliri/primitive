package process

import (
	"errors"
	"math"
	"strconv"
	"strings"

	"github.com/deliri/primitive/v2026/core"
)

// ParseGoMemoryLimit decodes Go's GOMEMLIMIT environment grammar. Empty and
// "off" mean math.MaxInt64; zero is a valid soft limit. Parsing neither changes
// the runtime nor admits memory or establishes a hard process-tree ceiling.
func ParseGoMemoryLimit(text string) (core.ByteLength, error) {
	if text == "" || text == "off" {
		return core.NewByteLength(math.MaxInt64)
	}
	var shift uint
	if number, suffixed := strings.CutSuffix(text, "B"); suffixed {
		text = number
		if number, binary := strings.CutSuffix(text, "i"); binary {
			if len(number) == 0 {
				return core.ByteLength{}, core.ErrProcessContract
			}
			binaryShift, err := goMemoryLimitBinaryUnitShift(number[len(number)-1])
			if err != nil {
				return core.ByteLength{}, err
			}
			shift = binaryShift
			text = number[:len(number)-1]
		}
	}
	value, err := strconv.ParseInt(text, 10, 64)
	if err != nil || value < 0 {
		return core.ByteLength{}, errors.Join(core.ErrProcessContract, err)
	}
	if value > math.MaxInt64>>shift {
		return core.ByteLength{}, errors.Join(core.ErrProcessContract, core.ErrNumericOverflow)
	}
	return core.NewByteLength(uint64(value) << shift)
}

func goMemoryLimitBinaryUnitShift(unit byte) (uint, error) {
	switch unit {
	case 'K':
		return 10, nil
	case 'M':
		return 20, nil
	case 'G':
		return 30, nil
	case 'T':
		return 40, nil
	default:
		return 0, core.ErrProcessContract
	}
}
