package process

import (
	"errors"
	"io"
	"math"
	"math/big"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// This table pressures native numeric domains and row framing, not a process
// count quota. The native source emits decimal PGID, RSS-in-KiB and CPU time.
func TestGroupMemoryFoldLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, rows string
		want       uint64
		wantErr    error
	}{
		{name: "matched resident page", rows: "42 4 0:00.00\n", want: 4096},
		{name: "sum separate members", rows: "42 1 0:00.00\n42 2 00:00:00\n", want: 3072},
		{name: "unrelated row excluded", rows: "41 9 0:00.00\n42 1 0:00.00\n", want: 1024},
		{name: "unrelated row after member", rows: "42 2 0:00.00\n43 9 0:00.00\n", want: 2048},
		{name: "native zero group ignored", rows: "0 9 0:00.00\n42 3 0:00.00\n", want: 3072},
		{name: "native init group ignored", rows: "1 9 0:00.00\n42 1 0:00.00\n", want: 1024},
		{name: "tabs accepted", rows: "\t42\t5\t0:00.00\t\n", want: 5120},
		{name: "CRLF accepted", rows: "42 6 0:00.00\r\n", want: 6144},
		{name: "EOF completes row", rows: "42 7 0:00.00", want: 7168},
		{name: "leading zeros numeric", rows: "00042 00008 0:00.00\n", want: 8192},
		{name: "empty observation", rows: ""},
		{name: "whitespace observation", rows: " \t\n\r\n"},
		{name: "zero RSS", rows: "42 0 0:00.00\n"},
		{name: "absent group", rows: "43 7 0:00.00\n"},
		{name: "PGID minimum native zero", rows: "0 0 0:00.00\n"},
		{name: "PGID maximum minus one", rows: "2147483646 1 0:00.00\n"},
		{name: "PGID maximum", rows: "2147483647 1 0:00.00\n"},
		{name: "PGID maximum plus one", rows: "2147483648 1 0:00.00\n", wantErr: core.ErrProcessObservation},
		{name: "RSS byte domain maximum KiB", rows: "42 9007199254740991 0:00.00\n", want: 9223372036854774784},
		{name: "RSS byte domain plus one", rows: "42 9007199254740992 0:00.00\n", wantErr: core.ErrProcessObservation},
		{name: "RSS multiply maximum", rows: "43 18014398509481983 0:00.00\n"},
		{name: "RSS multiply overflow", rows: "43 18014398509481984 0:00.00\n", wantErr: core.ErrProcessObservation},
		{name: "RSS decimal maximum", rows: "43 18446744073709551615 0:00.00\n", wantErr: core.ErrProcessObservation},
		{name: "decimal overflow", rows: "43 18446744073709551616 0:00.00\n", wantErr: core.ErrProcessObservation},
		{name: "sum signed byte overflow", rows: "42 9007199254740991 0:00.00\n42 1 0:00.00\n", wantErr: core.ErrProcessObservation},
		{name: "sum unsigned overflow", rows: "42 18014398509481983 0:00.00\n42 1 0:00.00\n", wantErr: core.ErrProcessObservation},
		{name: "header rejected", rows: "PGID RSS TIME\n", wantErr: core.ErrProcessObservation},
		{name: "missing RSS at newline", rows: "42\n", wantErr: core.ErrProcessObservation},
		{name: "missing time at newline", rows: "42 1\n", wantErr: core.ErrProcessObservation},
		{name: "missing time at EOF", rows: "42 1 ", wantErr: core.ErrProcessObservation},
		{name: "fourth column rejected", rows: "42 1 0:00.00 2\n", wantErr: core.ErrProcessObservation},
		{name: "minus PGID rejected", rows: "-42 1 0:00.00\n", wantErr: core.ErrProcessObservation},
		{name: "time mark in PGID rejected", rows: "4:2 1 0:00.00\n", wantErr: core.ErrProcessObservation},
		{name: "plus RSS rejected", rows: "42 +1 0:00.00\n", wantErr: core.ErrProcessObservation},
		{name: "fraction RSS rejected", rows: "42 1.5 0:00.00\n", wantErr: core.ErrProcessObservation},
		{name: "hex rejected", rows: "42 0xff 0:00.00\n", wantErr: core.ErrProcessObservation},
		{name: "NUL rejected", rows: "42 1 0:00.00\x00\n", wantErr: core.ErrProcessObservation},
		{name: "vertical tab rejected", rows: "42\v1 0:00.00\n", wantErr: core.ErrProcessObservation},
		{name: "split row not silently joined", rows: "42 1\n0:00.00\n", wantErr: core.ErrProcessObservation},
		{name: "bad suffix removes partial proof", rows: "42 1 0:00.00\n42 x 0:00.00\n", wantErr: core.ErrProcessObservation},
		{name: "huge whitespace stays valid", rows: strings.Repeat(" ", 65537) + "42 1 0:00.00\n", want: 1024},
		{name: "many zero digits stay numeric", rows: "42 " + strings.Repeat("0", 65537) + "1 0:00.00\n", want: 1024},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := foldGroupUsage(t, tc.rows)
			if !errors.Is(err, tc.wantErr) || got.Resident.Uint64() != tc.want || got.CPUTime.Nanoseconds() != 0 {
				t.Fatalf("fold=(%d,%d,%v), want (%d,0,%v)", got.Resident.Uint64(), got.CPUTime.Nanoseconds(), err, tc.want, tc.wantErr)
			}
		})
	}
}

// Native time columns: procps prints [DD-]HH:MM:SS and macOS ps prints
// M:SS.cc with unbounded minutes. Rows target group 42 with zero RSS.
func TestGroupUsageTimeLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, rows string
		centis     int64
		wantErr    error
	}{
		{name: "macOS zero", rows: "42 0 0:00.00\n"},
		{name: "macOS centiseconds", rows: "42 0 0:01.75\n", centis: 175},
		{name: "macOS unbounded minutes", rows: "42 0 110:01.40\n", centis: 660140},
		{name: "macOS space padded", rows: "42 0   0:01.75\n", centis: 175},
		{name: "macOS seconds without fraction", rows: "42 0 1:05\n", centis: 6500},
		{name: "procps one second", rows: "42 0 00:00:01\n", centis: 100},
		{name: "procps day maximum clock", rows: "42 0 23:59:59\n", centis: 8639900},
		{name: "procps one day", rows: "42 0 1-00:00:00\n", centis: 8640000},
		{name: "procps days", rows: "42 0 12-03:04:05\n", centis: (12*86400 + 3*3600 + 4*60 + 5) * 100},
		{name: "members summed others excluded", rows: "42 0 0:01.50\n43 0 9:00.00\n42 0 00:00:02\n", centis: 350},
		{name: "duration maximum", rows: "42 0 153722867:16.85\n", centis: 922337203685},
		{name: "duration maximum plus one", rows: "42 0 153722867:16.86\n", wantErr: core.ErrProcessObservation},
		{name: "sum beyond duration", rows: "42 0 153722867:16.85\n42 0 0:00.01\n", wantErr: core.ErrProcessObservation},
		{name: "unrelated huge time not summed", rows: "43 0 153722867:16.86\n42 0 0:00.01\n", centis: 1},
		{name: "clock overflow refused", rows: "43 0 18446744073709551615:00\n", wantErr: core.ErrProcessObservation},
		{name: "centisecond overflow refused", rows: "43 0 213503982334601-00:00:00\n", wantErr: core.ErrProcessObservation},
		{name: "day overflow refused", rows: "43 0 213503982334602-00:00:00\n", wantErr: core.ErrProcessObservation},
		{name: "bare seconds refused", rows: "42 0 5\n", wantErr: core.ErrProcessObservation},
		{name: "one digit seconds refused", rows: "42 0 1:5\n", wantErr: core.ErrProcessObservation},
		{name: "three digit seconds refused", rows: "42 0 1:005\n", wantErr: core.ErrProcessObservation},
		{name: "sixty seconds refused", rows: "42 0 1:60.00\n", wantErr: core.ErrProcessObservation},
		{name: "sixty minutes refused", rows: "42 0 00:60:00\n", wantErr: core.ErrProcessObservation},
		{name: "twenty four hours refused", rows: "42 0 24:00:00\n", wantErr: core.ErrProcessObservation},
		{name: "days without hours refused", rows: "42 0 1-02:03\n", wantErr: core.ErrProcessObservation},
		{name: "trailing separator after a full clock refused", rows: "42 1 1:02:03:\n", wantErr: core.ErrProcessObservation},
		{name: "fourth clock place refused", rows: "42 0 1:02:03:04\n", wantErr: core.ErrProcessObservation},
		{name: "two day marks refused", rows: "42 0 1-2-03:04:05\n", wantErr: core.ErrProcessObservation},
		{name: "four clock places refused", rows: "42 0 00:00:00:00\n", wantErr: core.ErrProcessObservation},
		{name: "one fraction digit refused", rows: "42 0 0:00.0\n", wantErr: core.ErrProcessObservation},
		{name: "three fraction digits refused", rows: "42 0 0:00.000\n", wantErr: core.ErrProcessObservation},
		{name: "fraction on procps clock refused", rows: "42 0 00:00:00.00\n", wantErr: core.ErrProcessObservation},
		{name: "fraction with days refused", rows: "42 0 1-00:00.00\n", wantErr: core.ErrProcessObservation},
		{name: "second fraction refused", rows: "42 0 0:00.00.00\n", wantErr: core.ErrProcessObservation},
		{name: "empty lead refused", rows: "42 0 :00\n", wantErr: core.ErrProcessObservation},
		{name: "empty place refused", rows: "42 0 0::00\n", wantErr: core.ErrProcessObservation},
		{name: "dangling colon refused", rows: "42 0 0:\n", wantErr: core.ErrProcessObservation},
		{name: "dangling colon at EOF refused", rows: "42 0 0:", wantErr: core.ErrProcessObservation},
		{name: "dangling point refused", rows: "42 0 0:00.\n", wantErr: core.ErrProcessObservation},
		{name: "point without clock refused", rows: "42 0 0.00\n", wantErr: core.ErrProcessObservation},
		{name: "dangling day refused", rows: "42 0 1-\n", wantErr: core.ErrProcessObservation},
		{name: "negative time refused", rows: "42 0 -1:00\n", wantErr: core.ErrProcessObservation},
		{name: "comma decimal refused", rows: "42 0 0:00,00\n", wantErr: core.ErrProcessObservation},
		{name: "bad time removes partial proof", rows: "42 0 0:01.00\n42 0 0:x\n", wantErr: core.ErrProcessObservation},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := foldGroupUsage(t, tc.rows)
			if !errors.Is(err, tc.wantErr) || got.CPUTime.Nanoseconds() != tc.centis*10_000_000 || got.Resident.Uint64() != 0 {
				t.Fatalf("fold=(%d,%d,%v), want (0,%d,%v)", got.Resident.Uint64(), got.CPUTime.Nanoseconds(), err, tc.centis*10_000_000, tc.wantErr)
			}
		})
	}
}

// foldGroupUsage proves one outcome under byte, odd and page fragmentation and
// that a refused writer stays refused.
func foldGroupUsage(t *testing.T, rows string) (GroupUsage, error) {
	t.Helper()
	var first GroupUsage
	var firstErr error
	for i, window := range []int{1, 7, 4096} {
		fold := groupUsageFold{target: 42}
		_, writeErr := io.CopyBuffer(&fold, struct{ io.Reader }{strings.NewReader(rows)}, make([]byte, window))
		got, finishErr := fold.finish()
		err := errors.Join(writeErr, finishErr)
		if err != nil {
			if !errors.Is(err, core.ErrProcessObservation) || got != (GroupUsage{}) {
				t.Fatalf("window %d refusal=(%v,%v), want zero typed refusal", window, got, err)
			}
			if n, resumeErr := fold.Write([]byte("42 1 0:00.00\n")); n != 0 || !errors.Is(resumeErr, core.ErrProcessObservation) {
				t.Fatalf("failed writer resumed=(%d,%v)", n, resumeErr)
			}
		}
		if i == 0 {
			first, firstErr = got, err
		} else if got != first || (err == nil) != (firstErr == nil) {
			t.Fatalf("window %d=(%v,%v), want window 1 (%v,%v)", window, got, err, first, firstErr)
		}
	}
	return first, firstErr
}

func TestGroupMemoryFoldStreamsGeneratedRows(t *testing.T) {
	t.Parallel()
	for _, count := range []uint64{4095, 4096, 4097, 100000} {
		fold := groupUsageFold{target: 42}
		for range count {
			if _, err := io.WriteString(&fold, "0 9 0:00.00\n42 1 00:00:01\n43 9 1-00:00:00\n"); err != nil {
				t.Fatal(err)
			}
		}
		got, err := fold.finish()
		if err != nil || got.Resident.Uint64() != count*1024 || got.CPUTime.Nanoseconds() != int64(count)*1_000_000_000 {
			t.Fatalf("%d rows=(%d,%d,%v), want %d", count, got.Resident.Uint64(), got.CPUTime.Nanoseconds(), err, count*1024)
		}
	}
}

// Numeric seeds are emitted from typed group/RSS/time facts. The oracle uses
// an independent standard-library row parser and arithmetic conservation.
func FuzzGroupMemoryNativeRows(f *testing.F) {
	f.Add(uint32(42), uint64(7), uint64(3), uint32(61), uint8(1))
	f.Add(uint32(0), uint64(0), uint64(0), uint32(0), uint8(0))
	f.Add(uint32(core.ProcessPOSIXIdentityMaximum), uint64(math.MaxInt64/1024), uint64(1), uint32(math.MaxUint32), uint8(7))
	f.Fuzz(func(t *testing.T, group uint32, rss, second uint64, seconds uint32, window uint8) {
		fold := groupUsageFold{target: 42}
		clock := strconv.FormatUint(uint64(seconds/60), 10) + ":" + strconv.FormatUint(uint64(seconds%60/10), 10) + strconv.FormatUint(uint64(seconds%10), 10) + ".00"
		text := strconv.FormatUint(uint64(group), 10) + " " + strconv.FormatUint(rss, 10) + " " + clock + "\n42 " + strconv.FormatUint(second, 10) + " 00:00:01\n"
		step := int(window) + 1
		var writeErr error
		for start := 0; start < len(text); start += step {
			if _, writeErr = fold.Write([]byte(text[start:min(start+step, len(text))])); writeErr != nil {
				break
			}
		}
		got, finishErr := fold.finish()
		err := errors.Join(writeErr, finishErr)
		want, cpu := uint64(0), int64(1_000_000_000)
		refused := group > core.ProcessPOSIXIdentityMaximum || rss > math.MaxUint64/1024 || second > math.MaxInt64/1024
		if !refused {
			want = second * 1024
			if group == 42 {
				cpu += int64(seconds) * 1_000_000_000
				if rss > (math.MaxInt64-want)/1024 {
					refused = true
				} else {
					want += rss * 1024
				}
			}
		}
		if refused {
			if !errors.Is(err, core.ErrProcessObservation) || got != (GroupUsage{}) {
				t.Fatalf("refused=(%v,%v), want zero typed refusal", got, err)
			}
			return
		}
		if err != nil || got.Resident.Uint64() != want || got.CPUTime.Nanoseconds() != cpu {
			t.Fatalf("fold=(%d,%d,%v), want (%d,%d,nil)", got.Resident.Uint64(), got.CPUTime.Nanoseconds(), err, want, cpu)
		}
	})
}

func FuzzGroupMemoryRowRepresentation(f *testing.F) {
	group := ProcessIdentity(42)
	if err := validateObservedGroup(group); err != nil {
		f.Fatal(err)
	}
	f.Add(strconv.FormatUint(uint64(group), 10)+" 7 1:02.03\n0 1 00:00:01\n42 3 1-23:59:59\n", uint8(7))
	f.Add("42 -1 0:00.00\n", uint8(0))
	f.Add("42 1 1:02:03:\n", uint8(3))
	f.Add("42 1 0:00.00\n42 x 0:00.00\n", uint8(1))
	f.Add("42 1 24:00:00\n", uint8(2))
	f.Add("42 1 153722867:16.86\n", uint8(3))
	f.Add("", uint8(0))
	f.Fuzz(func(t *testing.T, text string, window uint8) {
		fold := groupUsageFold{target: group}
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
			if !errors.Is(err, core.ErrProcessObservation) || got != (GroupUsage{}) {
				t.Fatalf("refusal=(%v,%v), want zero typed refusal", got, err)
			}
			return
		}
		// Independent row/token framing, strconv and big-integer arithmetic;
		// no aggregate process table or copy of the input is retained.
		want := uint64(0)
		cpu := new(big.Int)
		for row := range strings.SplitSeq(text, "\n") {
			fields := strings.Fields(row)
			if len(fields) == 0 {
				continue
			}
			if len(fields) != 3 {
				t.Fatalf("accepted impossible native row %q", row)
			}
			id, idErr := strconv.ParseUint(fields[0], 10, 64)
			rss, rssErr := strconv.ParseUint(fields[1], 10, 64)
			centis, timeOK := oracleNativeCentiseconds(fields[2])
			if idErr != nil || rssErr != nil || !timeOK || id > uint64(core.ProcessPOSIXIdentityMaximum) || rss > math.MaxUint64/1024 || !centis.IsUint64() {
				t.Fatalf("accepted impossible native row %q", row)
			}
			if id == uint64(group) {
				if rss > (math.MaxInt64-want)/1024 {
					t.Fatalf("accepted overflow row %q", row)
				}
				want += rss * 1024
				cpu.Add(cpu, centis)
			}
		}
		cpu.Mul(cpu, big.NewInt(10_000_000))
		if !cpu.IsInt64() || got.Resident.Uint64() != want || got.CPUTime.Nanoseconds() != cpu.Int64() {
			t.Fatalf("sum=(%d,%d), want (%d,%s)", got.Resident.Uint64(), got.CPUTime.Nanoseconds(), want, cpu)
		}
	})
}

// oracleNativeCentiseconds independently admits procps [DD-]HH:MM:SS and
// macOS M:SS.cc and returns exact centiseconds.
func oracleNativeCentiseconds(field string) (*big.Int, bool) {
	days, clock, hasDays := strings.Cut(field, "-")
	if !hasDays {
		days, clock = "0", field
	}
	clock, fraction, hasFraction := strings.Cut(clock, ".")
	if !hasFraction {
		fraction = "00"
	}
	places := strings.Split(clock, ":")
	if len(places) < 2 || len(places) > 3 || hasDays && len(places) != 3 || hasFraction && (len(places) != 2 || hasDays) || len(fraction) != 2 {
		return nil, false
	}
	total := new(big.Int)
	for _, digits := range append([]string{days}, places...) {
		if _, err := strconv.ParseUint(digits, 10, 64); err != nil {
			return nil, false
		}
	}
	dayValue, _ := new(big.Int).SetString(days, 10)
	total.Mul(dayValue, big.NewInt(86400))
	clockValue := new(big.Int)
	for i, place := range places {
		value, _ := strconv.ParseUint(place, 10, 64)
		if i > 0 && (len(place) != 2 || value >= 60) || i == 0 && len(places) == 3 && value >= 24 {
			return nil, false
		}
		clockValue.Mul(clockValue, big.NewInt(60))
		clockValue.Add(clockValue, new(big.Int).SetUint64(value))
	}
	fractionValue, err := strconv.ParseUint(fraction, 10, 64)
	if err != nil {
		return nil, false
	}
	total.Add(total, clockValue)
	total.Mul(total, big.NewInt(100))
	return total.Add(total, new(big.Int).SetUint64(fractionValue)), true
}
