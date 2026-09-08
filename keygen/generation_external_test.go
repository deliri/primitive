package keygen_test

import (
	"errors"
	"math"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/keygen"
)

func TestSecretRequestRejectsEveryAdjacentAndExtremeInvalidSize(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		value uint64
		zero  bool
	}{
		{name: "unset byte count rejected", zero: true},
		{name: "one below minimum rejected", value: core.SecretMaterialMinimumBytes - 1},
		{name: "one above maximum rejected", value: core.SecretMaterialMaximumBytes + 1},
		{name: "maximum uint64 rejected", value: math.MaxUint64},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var count core.ByteCount
			if !tc.zero {
				var gotCountErr error
				count, gotCountErr = core.NewByteCount(tc.value)
				if gotCountErr != nil {
					t.Fatalf("core.NewByteCount(%d) error = %v, want nil", tc.value, gotCountErr)
				}
			}
			request := keygen.SecretRequest{Size: count}
			gotValidateErr := request.Validate()
			if !errors.Is(gotValidateErr, core.ErrKeygenContract) ||
				!errors.Is(gotValidateErr, core.ErrPrimitiveContract) {
				t.Fatalf("SecretRequest{%d}.Validate() error = %v, want %v and %v", tc.value, gotValidateErr, core.ErrKeygenContract, core.ErrPrimitiveContract)
			}
			got, gotErr := keygen.GenerateSecret(request)
			if got != (core.SecretMaterial{}) ||
				!errors.Is(gotErr, core.ErrKeygenContract) ||
				!errors.Is(gotErr, core.ErrPrimitiveContract) {
				t.Fatalf("GenerateSecret(%d) = (%v, %v), want (zero, %v and %v)", tc.value, got, gotErr, core.ErrKeygenContract, core.ErrPrimitiveContract)
			}
		})
	}
}
