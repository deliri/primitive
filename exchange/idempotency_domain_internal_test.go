package exchange

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// The grammar is a byte-wise conjunction and an independent extent bound.
// Exhaust the byte domain at the beginning, interior, and final position, then
// every admitted extent. These are finite-domain proofs, not quota counts.
func TestIdempotencyKeyWireDomainTable(t *testing.T) {
	t.Parallel()
	type keyCase struct {
		name    string
		input   string
		want    string
		wantErr error
	}
	cases := make([]keyCase, 0, 3*(math.MaxUint8+1)+IdempotencyKeyMaximumBytes+2)
	for raw := range math.MaxUint8 + 1 {
		for position := range 3 {
			input := []byte("key")
			input[position] = byte(raw)
			row := keyCase{name: fmt.Sprintf("octet_%02x_at_position_%d_preserves_exact_identity_or_refuses", raw, position), input: string(input), wantErr: core.ErrExchangeContract}
			if raw >= '!' && raw <= '~' {
				row.want, row.wantErr = string(input), nil
			}
			cases = append(cases, row)
		}
	}
	for size := range IdempotencyKeyMaximumBytes + 2 {
		input := strings.Repeat("~", size)
		row := keyCase{name: fmt.Sprintf("extent_%d_cannot_truncate_or_invent_identity", size), input: input, wantErr: core.ErrExchangeContract}
		if size > 0 && size <= IdempotencyKeyMaximumBytes {
			row.want, row.wantErr = input, nil
		}
		cases = append(cases, row)
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, gotErr := ParseIdempotencyKey(tc.input)
			if !errors.Is(gotErr, tc.wantErr) || got.String() != tc.want || got.IsZero() != (tc.wantErr != nil) {
				t.Fatalf("key parse = (%q, %v), want (%q, %v)", got.String(), gotErr, tc.want, tc.wantErr)
			}
			// Validate owns the rule independently of constructor execution.
			if err := (IdempotencyKey{value: tc.input}).Validate(); !errors.Is(err, tc.wantErr) {
				t.Fatalf("nominal key validation = %v, want %v", err, tc.wantErr)
			}
		})
	}
}
