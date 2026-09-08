package keygen_test

import (
	"errors"
	"math"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/keygen"
)

func requireByteCount(t *testing.T, value uint64) core.ByteCount {
	t.Helper()
	count, err := core.NewByteCount(value)
	if err != nil {
		t.Fatalf("core.NewByteCount(%d) error = %v, want nil", value, err)
	}
	return count
}

func TestRandomTokenRequestAdmitsOnlyBoundedSizes(t *testing.T) {
	t.Parallel()

	for size := uint64(1); size <= keygen.RandomTokenMaximumBytes; size++ {
		request := keygen.RandomTokenRequest{Size: requireByteCount(t, size)}
		if gotErr := request.Validate(); gotErr != nil {
			t.Fatalf("RandomTokenRequest{%d}.Validate() error = %v, want nil", size, gotErr)
		}
	}

	invalidSizes := []struct {
		wantErr error
		name    string
		size    uint64
	}{
		{name: "one above ceiling is rejected", size: keygen.RandomTokenMaximumBytes + 1, wantErr: core.ErrKeygenContract},
		{name: "maximum uint64 is rejected", size: math.MaxUint64, wantErr: core.ErrKeygenContract},
	}
	for _, tc := range invalidSizes {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			request := keygen.RandomTokenRequest{Size: requireByteCount(t, tc.size)}
			got, effectErr := keygen.RandomToken(request)
			raw, projectionErr := got.Bytes()
			if !errors.Is(effectErr, tc.wantErr) || raw != nil || !errors.Is(projectionErr, core.ErrKeygenContract) {
				t.Fatalf("RandomToken(refused) = (%x,%v,%v), want no projection and Core refusals", raw, effectErr, projectionErr)
			}
			gotErr := request.Validate()
			if !errors.Is(gotErr, tc.wantErr) ||
				!errors.Is(gotErr, core.ErrPrimitiveContract) {
				t.Fatalf("RandomTokenRequest{%d}.Validate() error = %v, want %v and %v", tc.size, gotErr, tc.wantErr, core.ErrPrimitiveContract)
			}
		})
	}
}

// The zero size is unrepresentable through ByteCount, which rejects it before
// RandomTokenRequest is even built. The zero-value request carries that zero
// ByteCount, so its refusal is the compiler-owned zero-size proof.
func TestRandomTokenRequestZeroValueIsRefused(t *testing.T) {
	t.Parallel()

	if err := (keygen.RandomTokenRequest{}).Validate(); !errors.Is(err, core.ErrKeygenContract) {
		t.Fatalf("RandomTokenRequest{}.Validate() error = %v, want %v", err, core.ErrKeygenContract)
	}
}

// TestTokenRefusesTheUndrawnZeroValue keeps a token that skipped the draw from
// circulating as one that happened: the zero value fails validation and hands
// back no bytes.
func TestTokenRefusesTheUndrawnZeroValue(t *testing.T) {
	t.Parallel()

	if err := (keygen.Token{}).Validate(); !errors.Is(err, core.ErrKeygenContract) {
		t.Fatalf("Token{}.Validate() error = %v, want %v", err, core.ErrKeygenContract)
	}
	if drawn, err := (keygen.Token{}).Bytes(); !errors.Is(err, core.ErrKeygenContract) || drawn != nil {
		t.Fatalf("Token{}.Bytes() = (%v, %v), want (nil, %v)", drawn, err, core.ErrKeygenContract)
	}
}

func TestRandomTokenRefusesAnUnboundedRequest(t *testing.T) {
	t.Parallel()

	refused, err := keygen.RandomToken(keygen.RandomTokenRequest{Size: requireByteCount(t, keygen.RandomTokenMaximumBytes+1)})
	if !errors.Is(err, core.ErrKeygenContract) {
		t.Fatalf("RandomToken(over ceiling) error = %v, want %v", err, core.ErrKeygenContract)
	}
	if gotErr := refused.Validate(); !errors.Is(gotErr, core.ErrKeygenContract) {
		t.Fatalf("RandomToken(over ceiling) token Validate() error = %v, want errors.Is %v", gotErr, core.ErrKeygenContract)
	}
	if _, err := keygen.RandomToken(keygen.RandomTokenRequest{}); !errors.Is(err, core.ErrKeygenContract) {
		t.Fatalf("RandomToken(zero) error = %v, want %v", err, core.ErrKeygenContract)
	}
}
