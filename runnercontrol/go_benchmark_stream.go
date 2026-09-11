package runnercontrol

import (
	"math"
	"unicode"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/runprotocol"
)

type goNumericToken struct {
	coefficient uint64
	scale       uint8
	dot         bool
	invalid     bool
	digits      bool
	fractional  bool
}

func (n *goNumericToken) consume(value rune) {
	if n.invalid {
		return
	}
	if value == '.' && !n.dot && n.digits {
		n.dot = true
		return
	}
	if value < '0' || value > '9' {
		n.invalid = true
		return
	}
	digit := uint64(value - '0')
	if n.coefficient > (math.MaxUint64-digit)/10 {
		n.invalid = true
		return
	}
	n.coefficient = n.coefficient*10 + digit
	n.digits = true
	if n.dot {
		n.fractional = true
		if n.scale == runprotocol.DecimalMeasurementScaleMaximum {
			n.invalid = true
			return
		}
		n.scale++
	}
}

type goBenchmarkStream struct {
	measurement runprotocol.BenchmarkMeasurement
	number      goNumericToken
	previous    goNumericToken
	token       [runprotocol.NameMaximumBytes]byte
	length      int
	position    uint8
	metrics     uint8
	active      bool
	oversized   bool
	malformed   bool
	found       bool
	waitingUnit bool
}

func (goNumericToken) runnerControlInternalFlow()    {}
func (goBenchmarkStream) runnerControlInternalFlow() {}

func (b *goBenchmarkStream) write(data string) {
	for _, value := range data {
		if unicode.IsSpace(value) {
			b.endToken()
			continue
		}
		b.active = true
		b.number.consume(value)
		if b.oversized {
			continue
		}
		width := utf8.RuneLen(value)
		if width > len(b.token)-b.length {
			b.oversized = true
			continue
		}
		b.length += utf8.EncodeRune(b.token[b.length:], value)
	}
}

func (b *goBenchmarkStream) endToken() {
	if !b.active {
		return
	}
	token := string(b.token[:b.length])
	switch b.position {
	case 0:
		name, err := runprotocol.NewName(token)
		b.measurement.Name = name
		b.malformed = b.oversized || err != nil
		b.position = 1
	case 1:
		b.measurement.Iterations = b.number.coefficient
		b.malformed = b.malformed || b.number.invalid || b.number.dot || b.number.coefficient == 0
		b.position = 2
	default:
		if !b.oversized {
			if b.waitingUnit {
				b.metric(token)
			} else if token == "ns/op" {
				b.found, b.malformed = true, true
			}
		}
		b.waitingUnit = !b.waitingUnit
	}
	b.previous, b.number = b.number, goNumericToken{}
	b.length, b.active, b.oversized = 0, false, false
}

func (b *goBenchmarkStream) metric(unit string) {
	var mask uint8
	switch unit {
	case "ns/op":
		mask = 1
		b.found = true
	case "B/op":
		mask = 2
	case "allocs/op":
		mask = 4
	default:
		return
	}
	// Conflicting or identical duplicate units are malformed producer records,
	// not an order-dependent first-match-wins measurement.
	if b.metrics&mask != 0 {
		b.malformed = true
	}
	b.metrics |= mask
	n := b.previous
	if n.invalid || !n.digits || n.dot && !n.fractional {
		b.malformed = true
	}
	if mask != 1 && n.dot {
		b.malformed = true
	}
	switch mask {
	case 1:
		for n.scale > 0 && n.coefficient%10 == 0 {
			n.coefficient /= 10
			n.scale--
		}
		b.measurement.NanosecondsPerOp = runprotocol.DecimalMeasurement{Coefficient: n.coefficient, Scale: n.scale}
	case 2:
		b.measurement.BytesPerOp = n.coefficient
	case 4:
		b.measurement.AllocationsPerOp = n.coefficient
	}
}

func (b *goBenchmarkStream) result() (runprotocol.BenchmarkMeasurement, bool, error) {
	if !b.found {
		return runprotocol.BenchmarkMeasurement{}, false, nil
	}
	if b.malformed || b.metrics != 7 {
		return runprotocol.BenchmarkMeasurement{}, false, observationFailure("go benchmark record has invalid or duplicate fields", core.ErrJSONContract)
	}
	if err := b.measurement.Validate(); err != nil {
		return runprotocol.BenchmarkMeasurement{}, false, err
	}
	return b.measurement, true, nil
}
