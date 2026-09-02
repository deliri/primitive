package keygen_test

import (
	"errors"
	"math"
	"slices"
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
		{name: "two above ceiling are rejected", size: keygen.RandomTokenMaximumBytes + 2, wantErr: core.ErrKeygenContract},
		{name: "one hundred twenty eight bytes are rejected", size: 128, wantErr: core.ErrKeygenContract},
		{name: "one kilobyte is rejected", size: 1024, wantErr: core.ErrKeygenContract},
		{name: "four kilobytes are rejected", size: 4096, wantErr: core.ErrKeygenContract},
		{name: "maximum uint8 is rejected", size: math.MaxUint8, wantErr: core.ErrKeygenContract},
		{name: "maximum uint16 is rejected", size: math.MaxUint16, wantErr: core.ErrKeygenContract},
		{name: "maximum uint32 is rejected", size: math.MaxUint32, wantErr: core.ErrKeygenContract},
		{name: "maximum int64 is rejected", size: math.MaxInt64, wantErr: core.ErrKeygenContract},
		{name: "maximum uint64 is rejected", size: math.MaxUint64, wantErr: core.ErrKeygenContract},
	}
	for _, tc := range invalidSizes {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			request := keygen.RandomTokenRequest{Size: requireByteCount(t, tc.size)}
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

func TestRandomTokenFillsTheExactRequestedExtent(t *testing.T) {
	t.Parallel()

	for size := uint64(1); size <= keygen.RandomTokenMaximumBytes; size++ {
		token, err := keygen.RandomToken(keygen.RandomTokenRequest{Size: requireByteCount(t, size)})
		if err != nil {
			t.Fatalf("RandomToken(%d) error = %v, want nil", size, err)
		}
		if err := token.Validate(); err != nil {
			t.Fatalf("RandomToken(%d).Validate() error = %v, want a drawn token", size, err)
		}
		drawn, err := token.Bytes()
		if err != nil {
			t.Fatalf("RandomToken(%d).Bytes() error = %v, want nil", size, err)
		}
		if got, want := uint64(len(drawn)), size; got != want {
			t.Fatalf("RandomToken(%d) length = %d, want %d", size, len(drawn), size)
		}
		wantSecond := append([]byte(nil), drawn...)
		if len(drawn) > 0 {
			drawn[0] ^= 0xff
		}
		second, gotSecondErr := token.Bytes()
		if gotSecondErr != nil {
			t.Fatalf("RandomToken(%d).Bytes(second) error = %v, want nil", size, gotSecondErr)
		}
		if len(second) != int(size) {
			t.Fatalf("RandomToken(%d).Bytes(second) length = %d, want %d", size, len(second), size)
		}
		if !slices.Equal(second, wantSecond) {
			t.Fatalf("RandomToken(%d).Bytes(second) = %x, want immutable projection %x", size, second, wantSecond)
		}
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

func TestRandomUint64UsesProductionCSPRNGBoundary(t *testing.T) {
	t.Parallel()

	_, gotErr := keygen.RandomUint64()
	if gotErr != nil {
		t.Fatalf("RandomUint64() error = %v, want nil", gotErr)
	}
}
