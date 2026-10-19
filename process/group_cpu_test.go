package process

import (
	"errors"
	"io"
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// This table pressures the native %CPU spelling and row framing. ps emits a
// decimal PGID and whole percent with at most one decimal digit; procps drops
// the decimal at 1000% and above. One tenth of a percent is one milli-CPU.
func TestGroupCPUFoldLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, rows string
		want       uint64
		wantErr    error
	}{
		{name: "matched tenth", rows: "42 0.1\n", want: 1},
		{name: "matched decimal percent", rows: "42 12.3\n", want: 123},
		{name: "one full CPU", rows: "42 100.0\n", want: 1000},
		{name: "procps whole percent", rows: "42 1050\n", want: 10500},
		{name: "sum separate members", rows: "42 1.5\n42 2.5\n", want: 40},
		{name: "unrelated row excluded", rows: "41 99.9\n42 1.0\n", want: 10},
		{name: "native zero group ignored", rows: "0 50.0\n42 0.3\n", want: 3},
		{name: "native init group ignored", rows: "1 50.0\n42 0.2\n", want: 2},
		{name: "tabs accepted", rows: "\t42\t5.0\t\n", want: 50},
		{name: "CRLF accepted", rows: "42 6.0\r\n", want: 60},
		{name: "EOF completes value", rows: "42 7.0", want: 70},
		{name: "leading zeros numeric", rows: "00042 008.0\n", want: 80},
		{name: "empty observation", rows: ""},
		{name: "whitespace observation", rows: " \t\n\r\n"},
		{name: "idle member", rows: "42 0.0\n"},
		{name: "absent group", rows: "43 7.0\n"},
		{name: "PGID maximum", rows: "2147483647 1.0\n"},
		{name: "PGID maximum plus one", rows: "2147483648 1.0\n", wantErr: core.ErrProcessObservation},
		{name: "whole percent multiply maximum", rows: "42 1844674407370955161\n", want: 18446744073709551610},
		{name: "whole percent multiply overflow", rows: "42 1844674407370955162\n", wantErr: core.ErrProcessObservation},
		{name: "decimal digit maximum", rows: "42 1844674407370955161.5\n", want: 18446744073709551615},
		{name: "decimal digit overflow", rows: "42 1844674407370955161.6\n", wantErr: core.ErrProcessObservation},
		{name: "sum overflow", rows: "42 1844674407370955161.5\n42 0.1\n", wantErr: core.ErrProcessObservation},
		{name: "unrelated overflow still refused", rows: "43 1844674407370955162\n", wantErr: core.ErrProcessObservation},
		{name: "header rejected", rows: "PGID %CPU\n", wantErr: core.ErrProcessObservation},
		{name: "two decimals rejected", rows: "42 1.25\n", wantErr: core.ErrProcessObservation},
		{name: "trailing point rejected", rows: "42 1.\n", wantErr: core.ErrProcessObservation},
		{name: "leading point rejected", rows: "42 .5\n", wantErr: core.ErrProcessObservation},
		{name: "second point rejected", rows: "42 1.2.3\n", wantErr: core.ErrProcessObservation},
		{name: "decimal PGID rejected", rows: "42.0 1.0\n", wantErr: core.ErrProcessObservation},
		{name: "comma decimal rejected", rows: "42 1,5\n", wantErr: core.ErrProcessObservation},
		{name: "missing value at newline", rows: "42\n", wantErr: core.ErrProcessObservation},
		{name: "missing value at EOF", rows: "42 ", wantErr: core.ErrProcessObservation},
		{name: "third column rejected", rows: "42 1.0 2\n", wantErr: core.ErrProcessObservation},
		{name: "minus value rejected", rows: "42 -1.0\n", wantErr: core.ErrProcessObservation},
		{name: "exponent rejected", rows: "42 1e3\n", wantErr: core.ErrProcessObservation},
		{name: "NUL rejected", rows: "42 1.0\x00\n", wantErr: core.ErrProcessObservation},
		{name: "split row not silently joined", rows: "42\n1.0\n", wantErr: core.ErrProcessObservation},
		{name: "bad suffix removes partial proof", rows: "42 1.0\n42 x\n", wantErr: core.ErrProcessObservation},
		{name: "huge whitespace stays valid", rows: strings.Repeat(" ", 65537) + "42 1.0\n", want: 10},
		{name: "many zero digits stay numeric", rows: "42 " + strings.Repeat("0", 65537) + "1.0\n", want: 10},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for _, window := range []int{1, 7, 4096} {
				fold := groupCPUFold{target: 42}
				_, writeErr := io.CopyBuffer(&fold, struct{ io.Reader }{strings.NewReader(tc.rows)}, make([]byte, window))
				got, finishErr := fold.finish()
				err := errors.Join(writeErr, finishErr)
				if !errors.Is(err, tc.wantErr) || got != tc.want {
					t.Fatalf("window %d fold=(%d,%v), want (%d,%v)", window, got, err, tc.want, tc.wantErr)
				}
				if tc.wantErr != nil {
					if n, err := fold.Write([]byte("42 1.0\n")); n != 0 || !errors.Is(err, core.ErrProcessObservation) {
						t.Fatalf("failed writer resumed=(%d,%v)", n, err)
					}
				}
			}
		})
	}
}

func TestGroupCPUFoldStreamsGeneratedRows(t *testing.T) {
	t.Parallel()
	for _, count := range []uint64{4095, 4096, 4097, 100000} {
		fold := groupCPUFold{target: 42}
		for range count {
			if _, err := io.WriteString(&fold, "0 9.9\n42 0.1\n43 9.9\n"); err != nil {
				t.Fatal(err)
			}
		}
		got, err := fold.finish()
		if err != nil || got != count {
			t.Fatalf("%d rows=(%d,%v), want %d", count, got, err, count)
		}
	}
}

// Seeds are emitted from typed group/percent facts in both native spellings.
// The oracle recomputes milli-CPUs with independent checked arithmetic.
func FuzzGroupCPUNativeRows(f *testing.F) {
	f.Add(uint32(42), uint64(12), uint8(3), true, uint64(1), uint8(0))
	f.Add(uint32(0), uint64(0), uint8(0), false, uint64(0), uint8(6))
	f.Add(uint32(core.ProcessPOSIXIdentityMaximum), uint64(math.MaxUint64/10), uint8(9), true, uint64(1), uint8(2))
	f.Fuzz(func(t *testing.T, group uint32, whole uint64, tenth uint8, decimal bool, second uint64, window uint8) {
		tenth %= 10
		value := strconv.FormatUint(whole, 10)
		if decimal {
			value += "." + strconv.FormatUint(uint64(tenth), 10)
		}
		text := strconv.FormatUint(uint64(group), 10) + " " + value + "\n42 " + strconv.FormatUint(second, 10) + "\n"
		fold := groupCPUFold{target: 42}
		step := int(window) + 1
		var writeErr error
		for start := 0; start < len(text); start += step {
			if _, writeErr = fold.Write([]byte(text[start:min(start+step, len(text))])); writeErr != nil {
				break
			}
		}
		got, finishErr := fold.finish()
		err := errors.Join(writeErr, finishErr)

		first, firstOK := oracleMilli(whole, uint64(tenth), decimal)
		last, lastOK := oracleMilli(second, 0, false)
		refused := group > core.ProcessPOSIXIdentityMaximum || !firstOK || !lastOK
		want := last
		if !refused && group == 42 {
			if first > math.MaxUint64-last {
				refused = true
			} else {
				want = first + last
			}
		}
		if refused {
			if !errors.Is(err, core.ErrProcessObservation) || got != 0 {
				t.Fatalf("%q=(%d,%v), want observation refusal", text, got, err)
			}
			return
		}
		if err != nil || got != want {
			t.Fatalf("%q=(%d,%v), want %d", text, got, err, want)
		}
	})
}

func oracleMilli(whole, tenth uint64, decimal bool) (uint64, bool) {
	if whole > math.MaxUint64/10 {
		return 0, false
	}
	scaled := whole * 10
	if decimal {
		if scaled > math.MaxUint64-tenth {
			return 0, false
		}
		scaled += tenth
	}
	return scaled, true
}
