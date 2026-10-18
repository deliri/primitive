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

// This table pressures native numeric domains and row framing, not a process
// count quota. The native source emits decimal PGID and RSS-in-KiB columns.
func TestGroupMemoryFoldLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, rows string
		want       uint64
		wantErr    error
	}{
		{name: "matched resident page", rows: "42 4\n", want: 4096},
		{name: "sum separate members", rows: "42 1\n42 2\n", want: 3072},
		{name: "unrelated row excluded", rows: "41 9\n42 1\n", want: 1024},
		{name: "unrelated row after member", rows: "42 2\n43 9\n", want: 2048},
		{name: "native zero group ignored", rows: "0 9\n42 3\n", want: 3072},
		{name: "native init group ignored", rows: "1 9\n42 1\n", want: 1024},
		{name: "tabs accepted", rows: "\t42\t5\t\n", want: 5120},
		{name: "CRLF accepted", rows: "42 6\r\n", want: 6144},
		{name: "EOF completes RSS", rows: "42 7", want: 7168},
		{name: "leading zeros numeric", rows: "00042 00008\n", want: 8192},
		{name: "empty observation", rows: ""},
		{name: "whitespace observation", rows: " \t\n\r\n"},
		{name: "zero RSS", rows: "42 0\n"},
		{name: "absent group", rows: "43 7\n"},
		{name: "PGID minimum native zero", rows: "0 0\n"},
		{name: "PGID maximum minus one", rows: "2147483646 1\n"},
		{name: "PGID maximum", rows: "2147483647 1\n"},
		{name: "PGID maximum plus one", rows: "2147483648 1\n", wantErr: core.ErrProcessObservation},
		{name: "RSS byte domain maximum KiB", rows: "42 9007199254740991\n", want: 9223372036854774784},
		{name: "RSS byte domain plus one", rows: "42 9007199254740992\n", wantErr: core.ErrProcessObservation},
		{name: "RSS multiply maximum", rows: "43 18014398509481983\n"},
		{name: "RSS multiply overflow", rows: "43 18014398509481984\n", wantErr: core.ErrProcessObservation},
		{name: "RSS decimal maximum", rows: "43 18446744073709551615\n", wantErr: core.ErrProcessObservation},
		{name: "decimal overflow", rows: "43 18446744073709551616\n", wantErr: core.ErrProcessObservation},
		{name: "sum signed byte overflow", rows: "42 9007199254740991\n42 1\n", wantErr: core.ErrProcessObservation},
		{name: "sum unsigned overflow", rows: "42 18014398509481983\n42 1\n", wantErr: core.ErrProcessObservation},
		{name: "header rejected", rows: "PGID RSS\n", wantErr: core.ErrProcessObservation},
		{name: "missing RSS at newline", rows: "42\n", wantErr: core.ErrProcessObservation},
		{name: "missing RSS at EOF", rows: "42 ", wantErr: core.ErrProcessObservation},
		{name: "third column rejected", rows: "42 1 2\n", wantErr: core.ErrProcessObservation},
		{name: "minus PGID rejected", rows: "-42 1\n", wantErr: core.ErrProcessObservation},
		{name: "plus RSS rejected", rows: "42 +1\n", wantErr: core.ErrProcessObservation},
		{name: "fraction rejected", rows: "42 1.5\n", wantErr: core.ErrProcessObservation},
		{name: "hex rejected", rows: "42 0xff\n", wantErr: core.ErrProcessObservation},
		{name: "NUL rejected", rows: "42 1\x00\n", wantErr: core.ErrProcessObservation},
		{name: "vertical tab rejected", rows: "42\v1\n", wantErr: core.ErrProcessObservation},
		{name: "split row not silently joined", rows: "42\n1\n", wantErr: core.ErrProcessObservation},
		{name: "bad suffix removes partial proof", rows: "42 1\n42 x\n", wantErr: core.ErrProcessObservation},
		{name: "huge whitespace stays valid", rows: strings.Repeat(" ", 65537) + "42 1\n", want: 1024},
		{name: "many zero digits stay numeric", rows: "42 " + strings.Repeat("0", 65537) + "1\n", want: 1024},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for _, window := range []int{1, 7, 4096} {
				fold := groupMemoryFold{target: 42}
				_, writeErr := io.CopyBuffer(&fold, struct{ io.Reader }{strings.NewReader(tc.rows)}, make([]byte, window))
				got, finishErr := fold.finish()
				err := errors.Join(writeErr, finishErr)
				if !errors.Is(err, tc.wantErr) || got.Uint64() != tc.want {
					t.Fatalf("window %d fold=(%d,%v), want (%d,%v)", window, got.Uint64(), err, tc.want, tc.wantErr)
				}
				if tc.wantErr != nil {
					if n, err := fold.Write([]byte("42 1\n")); n != 0 || !errors.Is(err, core.ErrProcessObservation) {
						t.Fatalf("failed writer resumed=(%d,%v)", n, err)
					}
				}
			}
		})
	}
}

func TestGroupMemoryFoldStreamsGeneratedRows(t *testing.T) {
	t.Parallel()
	for _, count := range []uint64{4095, 4096, 4097, 100000} {
		fold := groupMemoryFold{target: 42}
		for range count {
			if _, err := io.WriteString(&fold, "0 9\n42 1\n43 9\n"); err != nil {
				t.Fatal(err)
			}
		}
		got, err := fold.finish()
		if err != nil || got.Uint64() != count*1024 {
			t.Fatalf("%d rows=(%d,%v), want %d", count, got.Uint64(), err, count*1024)
		}
	}
}

// Numeric seeds are emitted from typed group/RSS facts. The oracle uses an
// independent standard-library row parser and arithmetic conservation.
func FuzzGroupMemoryNativeRows(f *testing.F) {
	f.Add(uint32(42), uint64(7), uint64(3), uint8(1))
	f.Add(uint32(0), uint64(0), uint64(0), uint8(0))
	f.Add(uint32(core.ProcessPOSIXIdentityMaximum), uint64(math.MaxInt64/1024), uint64(1), uint8(7))
	f.Fuzz(func(t *testing.T, group uint32, rss, second uint64, window uint8) {
		fold := groupMemoryFold{target: 42}
		text := strconv.FormatUint(uint64(group), 10) + " " + strconv.FormatUint(rss, 10) + "\n42 " + strconv.FormatUint(second, 10) + "\n"
		step := int(window) + 1
		var writeErr error
		for start := 0; start < len(text); start += step {
			if _, writeErr = fold.Write([]byte(text[start:min(start+step, len(text))])); writeErr != nil {
				break
			}
		}
		got, finishErr := fold.finish()
		err := errors.Join(writeErr, finishErr)
		want := uint64(0)
		refused := group > core.ProcessPOSIXIdentityMaximum || rss > math.MaxUint64/1024 || second > math.MaxInt64/1024
		if !refused {
			want = second * 1024
			if group == 42 {
				if rss > (math.MaxInt64-want)/1024 {
					refused = true
				} else {
					want += rss * 1024
				}
			}
		}
		if refused {
			if !errors.Is(err, core.ErrProcessObservation) || got.Uint64() != 0 {
				t.Fatalf("refused=(%d,%v), want zero typed refusal", got.Uint64(), err)
			}
			return
		}
		if err != nil || got.Uint64() != want {
			t.Fatalf("fold=(%d,%v), want (%d,nil)", got.Uint64(), err, want)
		}
	})
}

func FuzzGroupMemoryRowRepresentation(f *testing.F) {
	group := ProcessIdentity(42)
	if err := validateObservedGroup(group); err != nil {
		f.Fatal(err)
	}
	f.Add(strconv.FormatUint(uint64(group), 10)+" 7\n0 1\n42 3\n", uint8(7))
	f.Add("42 -1\n", uint8(0))
	f.Add("42 1\n42 x\n", uint8(1))
	f.Add("", uint8(0))
	f.Fuzz(func(t *testing.T, text string, window uint8) {
		fold := groupMemoryFold{target: group}
		var writeErr error
		step := int(window) + 1
		for start := 0; start < len(text); start += step {
			if _, writeErr = fold.Write([]byte(text[start:min(start+step, len(text))])); writeErr != nil {
				break
			}
		}
		got, finishErr := fold.finish()
		err := errors.Join(writeErr, finishErr)
		if err != nil {
			if !errors.Is(err, core.ErrProcessObservation) || got.Uint64() != 0 {
				t.Fatalf("refusal=(%d,%v), want zero typed refusal", got.Uint64(), err)
			}
			return
		}
		// Independent row/token framing and strconv arithmetic; no aggregate
		// process table or copy of the input is retained by the oracle.
		want := uint64(0)
		for row := range strings.SplitSeq(text, "\n") {
			columns := 0
			var id, rss uint64
			for field := range strings.FieldsSeq(row) {
				value, err := strconv.ParseUint(field, 10, 64)
				if err != nil {
					t.Fatalf("accepted native field %q: %v", field, err)
				}
				if columns == 0 {
					id = value
				} else if columns == 1 {
					rss = value
				}
				columns++
			}
			if columns == 0 {
				continue
			}
			if columns != 2 || id > uint64(core.ProcessPOSIXIdentityMaximum) || rss > math.MaxUint64/1024 {
				t.Fatalf("accepted impossible native row %q", row)
			}
			if id == uint64(group) {
				if rss > (math.MaxInt64-want)/1024 {
					t.Fatalf("accepted overflow row %q", row)
				}
				want += rss * 1024
			}
		}
		if got.Uint64() != want {
			t.Fatalf("sum=%d, want %d", got.Uint64(), want)
		}
	})
}
