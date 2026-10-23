package temporal_test

import (
	"bytes"
	json "encoding/json/v2"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestCivilDateJSONPublishedFieldRetainsDate(t *testing.T) {
	t.Parallel()
	date, err := temporal.NewCivilDate(temporal.CivilDateRequest{Year: 2000, Month: temporal.February, Day: 29})
	if err != nil {
		t.Fatal(err)
	}
	got, err := core.MarshalCanonicalJSONDocument(struct {
		Birthday temporal.CivilDate `json:"birthday"`
	}{Birthday: date})
	const want = `{"birthday":"2000-02-29"}`
	if err != nil || string(got) != want {
		t.Fatalf("published birthday JSON=(%s,%v), want %s", got, err, want)
	}
}

func TestCivilDateJSONAdmissionAndReceiverAtomicity(t *testing.T) {
	t.Parallel()
	maximumEscaped := `"\u0032\u0030\u0030\u0030\u002d\u0030\u0032\u002d\u0032\u0039"`
	rows := []struct {
		name, wire, want    string
		refused, parseCause bool
	}{
		{name: "ordinary date retains coordinates", wire: `"1984-12-13"`, want: "1984-12-13"},
		{name: "leap divisible by400 is valid", wire: `"2000-02-29"`, want: "2000-02-29"},
		{name: "leap divisible by4 is valid", wire: `"2024-02-29"`, want: "2024-02-29"},
		{name: "century last valid February day", wire: `"1900-02-28"`, want: "1900-02-28"},
		{name: "minimum native year remains supported", wire: `"0000-01-01"`, want: "0000-01-01"},
		{name: "year zero Gregorian leap day", wire: `"0000-02-29"`, want: "0000-02-29"},
		{name: "first positive year remains supported", wire: `"0001-01-01"`, want: "0001-01-01"},
		{name: "largest representable date", wire: `"9999-12-31"`, want: "9999-12-31"},
		{name: "JSON whitespace surrounds one token", wire: " \t\r\n\"2000-02-29\"\n", want: "2000-02-29"},
		{name: "escaped ASCII canonicalizes to plain date", wire: `"\u0032000-02-29"`, want: "2000-02-29"},
		{name: "maximum escaped profile extent remains valid", wire: maximumEscaped, want: "2000-02-29"},
		{name: "large outside whitespace never becomes a flow quota", wire: strings.Repeat(" ", 65536) + maximumEscaped + "\n", want: "2000-02-29"},
		{name: "calendar nonleap day refuses without mutating", wire: `"2001-02-29"`, refused: true, parseCause: true},
		{name: "century nonleap day refuses", wire: `"1900-02-29"`, refused: true, parseCause: true},
		{name: "month above calendar domain refuses", wire: `"2000-13-01"`, refused: true, parseCause: true},
		{name: "month below calendar domain refuses", wire: `"2000-00-01"`, refused: true, parseCause: true},
		{name: "day below calendar domain refuses", wire: `"2000-01-00"`, refused: true, parseCause: true},
		{name: "day above calendar domain refuses", wire: `"2000-01-32"`, refused: true, parseCause: true},
		{name: "short month cannot normalize overflow", wire: `"2000-04-31"`, refused: true, parseCause: true},
		{name: "negative year is outside four digit profile", wire: `"-001-01-01"`, refused: true, parseCause: true},
		{name: "year above four digit profile refuses", wire: `"10000-01-01"`, refused: true, parseCause: true},
		{name: "truncated date refuses", wire: `"2000-02"`, refused: true, parseCause: true},
		{name: "timestamp cannot become date only", wire: `"2000-02-29T00:00:00Z"`, refused: true, parseCause: true},
		{name: "timezone cannot become date only", wire: `"2000-02-29Z"`, refused: true, parseCause: true},
		{name: "short year refuses", wire: `"200-02-29"`, refused: true, parseCause: true},
		{name: "noncanonical short month refuses", wire: `"2000-2-29"`, refused: true, parseCause: true},
		{name: "date text leading space refuses", wire: `" 2000-02-29"`, refused: true, parseCause: true},
		{name: "Unicode lookalike digits refuse", wire: `"２０００-02-29"`, refused: true, parseCause: true},
		{name: "empty JSON string refuses", wire: `""`, refused: true, parseCause: true},
		{name: "null is not a date", wire: `null`, refused: true},
		{name: "number is not a date", wire: `20000229`, refused: true},
		{name: "boolean is not a date", wire: `true`, refused: true},
		{name: "object is not a date", wire: `{}`, refused: true},
		{name: "array is not a date", wire: `[]`, refused: true},
		{name: "missing JSON input refuses", refused: true},
		{name: "unterminated JSON string refuses", wire: `"2000-02-29`, refused: true},
		{name: "trailing second token refuses", wire: `"2000-02-29" "2001-01-01"`, refused: true},
		{name: "invalid JSON escape refuses", wire: `"2000\x2d02-29"`, refused: true},
		{name: "lone surrogate refuses", wire: `"\ud800"`, refused: true},
		{name: "invalid UTF8 refuses", wire: "\"\xff\"", refused: true},
		{name: "one below escaped maximum truncated token refuses", wire: maximumEscaped[:len(maximumEscaped)-1], refused: true},
		{name: "one above escaped maximum refuses before allocation", wire: maximumEscaped + "x", refused: true},
	}
	initial, err := temporal.NewCivilDate(temporal.CivilDateRequest{Year: 2026, Month: temporal.May, Day: 6})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range rows {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := initial
			err := got.UnmarshalJSON([]byte(tc.wire))
			if tc.refused {
				if !errors.Is(err, core.ErrJSONContract) || !errors.Is(err, core.ErrTemporalContract) || got != initial {
					t.Fatalf("date JSON refusal=(%v,%v), want unchanged %v and JSON/Temporal identities", got, err, initial)
				}
				var native *time.ParseError
				if errors.As(err, &native) != tc.parseCause {
					t.Fatalf("native date cause=%t (%v), want %t", errors.As(err, &native), err, tc.parseCause)
				}
				return
			}
			text, textErr := got.Text()
			if err != nil || textErr != nil || text != tc.want {
				t.Fatalf("date JSON=(%q,%v,%v), want %q/nil", text, err, textErr, tc.want)
			}
			canonical, err := got.MarshalJSON()
			want, wantErr := core.MarshalCanonicalJSONString(tc.want)
			if err != nil || wantErr != nil || !bytes.Equal(canonical, want) || len(canonical) != 12 {
				t.Fatalf("canonical date=(%s,%v), want %s/%v and12bytes", canonical, err, want, wantErr)
			}
			var roundTrip temporal.CivilDate
			if err := json.Unmarshal(canonical, &roundTrip); err != nil || roundTrip != got {
				t.Fatalf("native JSON roundtrip=(%v,%v), want %v", roundTrip, err, got)
			}
		})
	}
}

func TestCivilDateJSONZeroAndOptionalField(t *testing.T) {
	t.Parallel()
	data, err := (temporal.CivilDate{}).MarshalJSON()
	if !errors.Is(err, core.ErrTemporalContract) || data != nil {
		t.Fatalf("unset date MarshalJSON=(%s,%v), want nil/Temporal refusal", data, err)
	}
	var receiver *temporal.CivilDate
	err = receiver.UnmarshalJSON([]byte(`"2000-02-29"`))
	if !errors.Is(err, core.ErrJSONContract) || !errors.Is(err, core.ErrTemporalContract) {
		t.Fatalf("nil date receiver error=%v, want JSON/Temporal refusal", err)
	}
	data, err = core.MarshalCanonicalJSONDocument(struct {
		Birthday temporal.CivilDate `json:"birthday,omitzero"`
	}{})
	if err != nil || string(data) != "{}" {
		t.Fatalf("absent optional birthday=(%s,%v), want empty object", data, err)
	}
}

func TestCivilDateJSONNativeParseCause(t *testing.T) {
	t.Parallel()
	var date temporal.CivilDate
	err := date.UnmarshalJSON([]byte(`"2026-02-30"`))
	var native *time.ParseError
	if !errors.Is(err, core.ErrJSONContract) || !errors.Is(err, core.ErrTemporalContract) || !errors.As(err, &native) || date != (temporal.CivilDate{}) {
		t.Fatalf("calendar refusal=(%v,%v), want zero and native ParseError/JSON/Temporal", date, err)
	}
}
