package process_test

import (
	"errors"
	"math"
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
)

func FuzzGoMemoryLimitNativeGrammar(f *testing.F) {
	for _, text := range []string{"", "off", "0", "+1", "-0", "0001", "64B", "1KiB", "64MiB", "1GiB", "1TiB", "-0TiB", "9223372036854775807", "9223372036854775808", "9007199254740991KiB", "9007199254740992KiB", "8796093022207MiB", "8796093022208MiB", "8589934591GiB", "8589934592GiB", "8388607TiB", "8388608TiB", "-1", "1.5GiB", "1MB", "1kiB", "MiB", "iB", "B", "1PiB", " 64MiB", "64MiB\n", "64\x00MiB", "\xff"} {
		f.Add(text)
	}
	// This oracle uses independent lexical admission and arbitrary-precision
	// arithmetic, rather than the production suffix walk and checked shift.
	grammar := regexp.MustCompile(`^([+-]?[0-9]+)(B|KiB|MiB|GiB|TiB)?$`)
	f.Fuzz(func(t *testing.T, text string) {
		want := big.NewInt(math.MaxInt64)
		valid := text == "" || text == "off"
		if !valid {
			parts := grammar.FindStringSubmatch(text)
			if parts != nil {
				number, parsed := new(big.Int).SetString(parts[1], 10)
				if parsed && number.Sign() >= 0 {
					power := int64(0)
					if len(parts[2]) == 3 {
						power = int64(strings.IndexByte("KMGT", parts[2][0]) + 1)
					}
					multiplier := new(big.Int).Exp(big.NewInt(1024), big.NewInt(power), nil)
					want.Mul(number, multiplier)
					valid = want.IsInt64()
				}
			}
		}
		got, err := process.ParseGoMemoryLimit(text)
		if !valid {
			if !errors.Is(err, core.ErrProcessContract) || got != (core.ByteLength{}) {
				t.Fatalf("ParseGoMemoryLimit(%q)=(%v,%v), want zero and process contract refusal", text, got, err)
			}
			return
		}
		if err != nil || got.Validate() != nil || got.Uint64() != want.Uint64() {
			t.Fatalf("ParseGoMemoryLimit(%q)=(%d,%v), want %s/nil", text, got.Uint64(), err, want.String())
		}
		canonical := strconv.FormatUint(got.Uint64(), 10)
		second, err := process.ParseGoMemoryLimit(canonical)
		if err != nil || second != got {
			t.Fatalf("canonical ParseGoMemoryLimit(%q)=(%v,%v), want %v/nil", canonical, second, err, got)
		}
	})
}
