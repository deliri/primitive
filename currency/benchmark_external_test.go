package currency_test

import (
	"bytes"
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/currency"
)

func BenchmarkParseDecimal(b *testing.B) {
	b.ReportAllocs()
	for _, tc := range []struct {
		name   string
		code   currency.Code
		minor  int64
		trim   bool
		reject bool
	}{
		{name: "minimum-four-digit", code: currency.CodeCLF, minor: math.MinInt64},
		{name: "short-two-digit", code: currency.CodeCAD, minor: 1230, trim: true},
		{name: "overfull-fraction", code: currency.CodeCLF, minor: 1, reject: true},
	} {
		b.Run(tc.name, func(b *testing.B) {
			want, err := currency.New(tc.code, tc.minor)
			if err != nil {
				b.Fatalf("New fixture = %v, want nil", err)
			}
			raw, err := want.Decimal()
			if err != nil {
				b.Fatalf("Decimal fixture = %v, want nil", err)
			}
			if tc.trim {
				raw = strings.TrimRight(raw, "0")
			}
			var wantErr error
			if tc.reject {
				raw += "1"
				want = currency.Amount{}
				wantErr = core.ErrCurrencyDecimal
			}
			b.ReportAllocs()
			var got currency.Amount
			for b.Loop() {
				got, err = currency.Parse(tc.code, raw)
				if !errors.Is(err, wantErr) {
					b.Fatalf("Parse = %v, want %v", err, wantErr)
				}
			}
			if got != want {
				b.Fatalf("Parse result = %v, want %v", got, want)
			}
		})
	}
}

func BenchmarkFormatDecimal(b *testing.B) {
	value, err := currency.New(currency.CodeCLF, math.MinInt64)
	if err != nil {
		b.Fatalf("New fixture = %v, want nil", err)
	}
	b.ReportAllocs()
	var got string
	for b.Loop() {
		got, err = value.Decimal()
		if err != nil {
			b.Fatalf("Decimal = %v, want nil", err)
		}
	}
	const want = "-922337203685477.5808"
	if got != want {
		b.Fatalf("Decimal result = %q, want %q", got, want)
	}
}

func BenchmarkAmountJSON(b *testing.B) {
	b.ReportAllocs()
	value, err := currency.New(currency.CodeCLF, math.MinInt64)
	if err != nil {
		b.Fatalf("New fixture = %v, want nil", err)
	}
	wire, err := value.MarshalJSON()
	if err != nil || len(wire) != currency.AmountCanonicalJSONMaximumBytes {
		b.Fatalf("JSON fixture = (%d,%v), want maximum/nil", len(wire), err)
	}
	b.Run("encode-maximum", func(b *testing.B) {
		b.ReportAllocs()
		var got []byte
		for b.Loop() {
			got, err = value.MarshalJSON()
			if err != nil {
				b.Fatalf("MarshalJSON = %v, want nil", err)
			}
		}
		if !bytes.Equal(got, wire) {
			b.Fatalf("MarshalJSON result = %q, want %q", got, wire)
		}
	})
	b.Run("decode-maximum", func(b *testing.B) {
		b.ReportAllocs()
		var got currency.Amount
		for b.Loop() {
			if err := got.UnmarshalJSON(wire); err != nil {
				b.Fatalf("UnmarshalJSON = %v, want nil", err)
			}
		}
		if got != value {
			b.Fatalf("UnmarshalJSON result = %v, want %v", got, value)
		}
	})
}

func BenchmarkCheckedAdd(b *testing.B) {
	left, err := currency.New(currency.CodeJPY, math.MaxInt64-1)
	if err != nil {
		b.Fatalf("left fixture = %v, want nil", err)
	}
	right, err := currency.New(currency.CodeJPY, 1)
	if err != nil {
		b.Fatalf("right fixture = %v, want nil", err)
	}
	want, err := currency.New(currency.CodeJPY, math.MaxInt64)
	if err != nil {
		b.Fatalf("result fixture = %v, want nil", err)
	}
	b.ReportAllocs()
	var got currency.Amount
	for b.Loop() {
		got, err = left.Add(right)
		if err != nil {
			b.Fatalf("Add = %v, want nil", err)
		}
	}
	if got != want {
		b.Fatalf("Add result = %v, want %v", got, want)
	}
}

func BenchmarkParseCode(b *testing.B) {
	b.ReportAllocs()
	var got currency.Code
	var err error
	for b.Loop() {
		got, err = currency.ParseCode(currency.CodeTokenCLF)
		if err != nil {
			b.Fatalf("ParseCode = %v, want nil", err)
		}
	}
	if got != currency.CodeCLF {
		b.Fatalf("ParseCode result = %v, want %v", got, currency.CodeCLF)
	}
}
