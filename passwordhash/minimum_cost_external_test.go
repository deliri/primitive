package passwordhash_test

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/passwordhash"
	"golang.org/x/crypto/argon2"
)

func minimumCostLimits(index uint32) passwordhash.Limits {
	minimum := passwordhash.Parameters{MemoryKiB: 128 + index, Iterations: 2 + index%4, Parallelism: 2 + uint8(index/4%4)}
	return passwordhash.Limits{Minimum: minimum, MemoryKiB: minimum.MemoryKiB + 32, Iterations: minimum.Iterations + 1, Parallelism: minimum.Parallelism + 1, KeyBytes: 32, MaterialBytes: 64, SaltBytes: 32, ConcurrentCalls: 1}
}

func TestPasswordCostFloorRefusesNativeValidDowngrades(t *testing.T) {
	t.Parallel()
	for index := range uint32(64) {
		t.Run(fmt.Sprint(index), func(t *testing.T) {
			t.Parallel()
			limits := minimumCostLimits(index)
			engine, err := passwordhash.New(limits)
			if err != nil {
				t.Fatal(err)
			}
			request := passwordhash.Request{Parameters: limits.Minimum, Material: []byte("password material"), Salt: []byte("public salt"), KeyBytes: 32}
			key, err := engine.Derive(t.Context(), request)
			if err != nil {
				t.Fatal(err)
			}
			defer clear(key)
			want := argon2.IDKey(request.Material, request.Salt, request.Parameters.Iterations, request.Parameters.MemoryKiB, request.Parameters.Parallelism, request.KeyBytes)
			defer clear(want)
			if !bytes.Equal(key, want) {
				t.Fatal("floor changed native key")
			}
			for dimension := range 3 {
				weakened := request
				switch dimension {
				case 0:
					weakened.Parameters.MemoryKiB--
				case 1:
					weakened.Parameters.Iterations--
				case 2:
					weakened.Parameters.Parallelism--
				}
				if err := weakened.Parameters.Validate(); err != nil {
					t.Fatalf("fixture is not a native-valid downgrade: %v", err)
				}
				key, err := engine.Derive(t.Context(), weakened)
				defer clear(key)
				if key != nil || !errors.Is(err, core.ErrPasswordHashContract) {
					t.Fatalf("dimension %d escaped explicit floor: %v", dimension, err)
				}
			}
			missing := limits
			missing.Minimum = passwordhash.Parameters{}
			if engine, err := passwordhash.New(missing); engine != nil || !errors.Is(err, core.ErrPasswordHashContract) {
				t.Fatalf("missing floor admitted: %v", err)
			}
			inverted := limits
			inverted.MemoryKiB = limits.Minimum.MemoryKiB - 1
			if engine, err := passwordhash.New(inverted); engine != nil || !errors.Is(err, core.ErrPasswordHashContract) {
				t.Fatalf("inverted cost interval admitted: %v", err)
			}
		})
	}
}

func FuzzPasswordCostIntervalAdmission(f *testing.F) {
	f.Add(uint32(127), uint32(2), uint8(2))
	f.Add(uint32(128), uint32(2), uint8(2))
	f.Add(uint32(160), uint32(3), uint8(3))
	f.Add(uint32(161), uint32(3), uint8(3))
	f.Fuzz(func(t *testing.T, memory, iterations uint32, lanes uint8) {
		limits := minimumCostLimits(0)
		request := passwordhash.Request{Parameters: passwordhash.Parameters{MemoryKiB: memory, Iterations: iterations, Parallelism: lanes}, Material: []byte("password material"), Salt: []byte("public salt"), KeyBytes: 32}
		want := memory >= 128 && memory <= 160 && iterations >= 2 && iterations <= 3 && lanes >= 2 && lanes <= 3
		engine, err := passwordhash.New(limits)
		if err != nil {
			t.Fatal(err)
		}
		key, err := engine.Derive(t.Context(), request)
		defer clear(key)
		if !want {
			if key != nil || !errors.Is(err, core.ErrPasswordHashContract) {
				t.Fatalf("cost interval widened: %v", err)
			}
			return
		}
		if err != nil {
			t.Fatal(err)
		}
		direct := argon2.IDKey(request.Material, request.Salt, iterations, memory, lanes, request.KeyBytes)
		defer clear(direct)
		if !bytes.Equal(key, direct) {
			t.Fatal("admitted cost changed native key")
		}
	})
}
