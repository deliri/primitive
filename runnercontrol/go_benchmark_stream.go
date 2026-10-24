package runnercontrol

import (
	"math"
	"unicode"
	"unicode/utf8"

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
	name        [runprotocol.NameMaximumBytes]byte
	length      int
	nameLength  int
	position    uint8
	metrics     uint8
	active      bool
	oversized   bool
	malformed   bool
	nameInvalid bool
	found       bool
	waitingUnit bool
	// iterations holds when the second token is a positive integer count. It
	// alone marks a native result row: with ns/op suppressed and no other
	// metric, testing prints only the name and its count.
	iterations bool
}

func (goNumericToken) runnerControlInternalFlow()    {}
func (goBenchmarkStream) runnerControlInternalFlow() {}

// write consumes borrowed decoded UTF-8. ASCII avoids rune decoding; the
// whitespace test matches unicode.IsSpace on that range exactly.
func (b *goBenchmarkStream) write(data []byte) {
	for len(data) > 0 {
		value, width := rune(data[0]), 1
		if value >= utf8.RuneSelf {
			value, width = utf8.DecodeRune(data)
		}
		data = data[width:]
		if goBenchmarkSpace(value) {
			b.endToken()
			continue
		}
		b.active = true
		b.number.consume(value)
		if b.oversized {
			continue
		}
		size := utf8.RuneLen(value)
		if size > len(b.token)-b.length {
			b.oversized = true
			continue
		}
		b.length += utf8.EncodeRune(b.token[b.length:], value)
	}
}

func goBenchmarkSpace(value rune) bool {
	if value < utf8.RuneSelf {
		return value == ' ' || value >= '\t' && value <= '\r'
	}
	return unicode.IsSpace(value)
}

func (b *goBenchmarkStream) endToken() {
	if !b.active {
		return
	}
	token := b.token[:b.length]
	switch b.position {
	case 0:
		// Most output lines are not benchmark records; the name is only
		// materialized by result once a metric proves the line is one.
		b.nameLength = copy(b.name[:], token)
		b.nameInvalid = b.oversized
		b.position = 1
	case 1:
		b.measurement.Iterations = b.number.coefficient
		b.iterations = !b.number.invalid && !b.number.dot && b.number.coefficient != 0
		b.malformed = b.malformed || !b.iterations
		b.position = 2
	default:
		b.endMetricToken(token)
	}
	b.previous, b.number = b.number, goNumericToken{}
	b.length, b.active, b.oversized = 0, false, false
}

func (b *goBenchmarkStream) endMetricToken(token []byte) {
	if !b.oversized {
		if b.waitingUnit {
			b.metric(token)
		} else if string(token) == "ns/op" {
			b.found, b.malformed = true, true
		}
	}
	b.waitingUnit = !b.waitingUnit
}

func (b *goBenchmarkStream) metric(unit []byte) {
	mask := goBenchmarkUnitMask(unit)
	if mask == 0 {
		return
	}
	if mask == 1 {
		b.found = true
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
	b.projectMetric(mask, n)
}

func (b *goBenchmarkStream) projectMetric(mask uint8, n goNumericToken) {
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

func goBenchmarkUnitMask(unit []byte) uint8 {
	switch string(unit) {
	case "ns/op":
		return 1
	case "B/op":
		return 2
	case "allocs/op":
		return 4
	default:
		return 0
	}
}
