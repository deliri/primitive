package keygen_test

import (
	"bytes"
	"errors"
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

	cases := []struct {
		name    string
		size    uint64
		wantErr bool
	}{
		{name: "one byte is the smallest token", size: 1},
		{name: "two bytes sit one above the floor", size: 2},
		{name: "a sixteen byte nonce is admitted", size: 16},
		{name: "the ceiling is admitted", size: keygen.RandomTokenMaximumBytes},
		{name: "one below the ceiling is admitted", size: keygen.RandomTokenMaximumBytes - 1},
		{name: "one above the ceiling is rejected", size: keygen.RandomTokenMaximumBytes + 1, wantErr: true},
		{name: "far above the ceiling is rejected", size: 4096, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			request := keygen.RandomTokenRequest{Size: requireByteCount(t, tc.size)}
			err := request.Validate()
			if gotErr := err != nil; gotErr != tc.wantErr {
				t.Fatalf("RandomTokenRequest{%d}.Validate() error = %v, wantErr %t", tc.size, err, tc.wantErr)
			}
			if tc.wantErr && !errors.Is(err, core.ErrKeygenContract) {
				t.Fatalf("RandomTokenRequest{%d}.Validate() error = %v, want %v", tc.size, err, core.ErrKeygenContract)
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

	for _, size := range []uint64{1, 16, 32, keygen.RandomTokenMaximumBytes} {
		token, err := keygen.RandomToken(keygen.RandomTokenRequest{Size: requireByteCount(t, size)})
		if err != nil {
			t.Fatalf("RandomToken(%d) error = %v, want nil", size, err)
		}
		if uint64(len(token)) != size {
			t.Fatalf("RandomToken(%d) length = %d, want %d", size, len(token), size)
		}
	}
}

func TestRandomTokenRefusesAnUnboundedRequest(t *testing.T) {
	t.Parallel()

	_, err := keygen.RandomToken(keygen.RandomTokenRequest{Size: requireByteCount(t, keygen.RandomTokenMaximumBytes+1)})
	if !errors.Is(err, core.ErrKeygenContract) {
		t.Fatalf("RandomToken(over ceiling) error = %v, want %v", err, core.ErrKeygenContract)
	}
	if _, err := keygen.RandomToken(keygen.RandomTokenRequest{}); !errors.Is(err, core.ErrKeygenContract) {
		t.Fatalf("RandomToken(zero) error = %v, want %v", err, core.ErrKeygenContract)
	}
}

func TestRandomTokenDrawsAreDistinct(t *testing.T) {
	t.Parallel()

	const draws = 64
	seen := make([][]byte, 0, draws)
	for range draws {
		token, err := keygen.RandomToken(keygen.RandomTokenRequest{Size: requireByteCount(t, 32)})
		if err != nil {
			t.Fatalf("RandomToken(32) error = %v, want nil", err)
		}
		for _, prior := range seen {
			if bytes.Equal(prior, token) {
				t.Fatalf("RandomToken(32) repeated a draw; the production CSPRNG must not collide across %d draws", draws)
			}
		}
		seen = append(seen, token)
	}
}

func TestRandomUint64DrawsAreDistinct(t *testing.T) {
	t.Parallel()

	const draws = 64
	seen := make(map[uint64]struct{}, draws)
	for range draws {
		value, err := keygen.RandomUint64()
		if err != nil {
			t.Fatalf("RandomUint64() error = %v, want nil", err)
		}
		if _, repeated := seen[value]; repeated {
			t.Fatalf("RandomUint64() repeated %d; the production CSPRNG must not collide across %d draws", value, draws)
		}
		seen[value] = struct{}{}
	}
}
