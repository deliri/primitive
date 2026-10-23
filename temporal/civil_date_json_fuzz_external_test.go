package temporal_test

import (
	"bytes"
	json "encoding/json/v2"
	"errors"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

func FuzzCivilDateJSONSemanticClosure(f *testing.F) {
	for _, request := range []temporal.CivilDateRequest{{Year: 0, Month: temporal.January, Day: 1}, {Year: 2000, Month: temporal.February, Day: 29}, {Year: 9999, Month: temporal.December, Day: 31}} {
		date, err := temporal.NewCivilDate(request)
		if err != nil {
			f.Fatal(err)
		}
		encoded, err := date.MarshalJSON()
		if err != nil {
			f.Fatal(err)
		}
		f.Add(encoded)
	}
	for _, raw := range []string{`null`, `{}`, `"2001-02-29"`, `"2000-2-29"`, `"2000-02-29" true`, `"\u0032000-02-29"`, ``} {
		f.Add([]byte(raw))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 4096 {
			return
		}
		initial, err := temporal.NewCivilDate(temporal.CivilDateRequest{Year: 2026, Month: temporal.May, Day: 6})
		if err != nil {
			t.Fatal(err)
		}
		got := initial
		gotErr := got.UnmarshalJSON(data)
		var text string
		jsonErr := json.Unmarshal(data, &text)
		native, dateErr := time.Parse(time.DateOnly, text)
		wantAccepted := jsonErr == nil && dateErr == nil && native.Format(time.DateOnly) == text
		if (gotErr == nil) != wantAccepted {
			t.Fatalf("date JSON=(%v,%v), native JSON/date=(%q,%v,%v), want admitted=%t", got, gotErr, text, jsonErr, dateErr, wantAccepted)
		}
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrJSONContract) || !errors.Is(gotErr, core.ErrTemporalContract) || got != initial {
				t.Fatalf("date JSON refusal=(%v,%v), want unchanged/JSON/Temporal", got, gotErr)
			}
			return
		}
		if err := got.Validate(); err != nil {
			t.Fatalf("accepted date=%v Validate=%v, want nil", got, err)
		}
		year, month, day := native.Date()
		coordinates, err := got.Coordinates()
		if err != nil || coordinates.Year != int32(year) || int(coordinates.Month) != int(month) || int(coordinates.Day) != day {
			t.Fatalf("decoded coordinates=(%+v,%v), want native %d/%d/%d", coordinates, err, year, month, day)
		}
		canonical, err := got.MarshalJSON()
		if err != nil || len(canonical) != 12 {
			t.Fatalf("canonical date=(%s,%v), want12byte valid JSON", canonical, err)
		}
		var again temporal.CivilDate
		if err := again.UnmarshalJSON(canonical); err != nil || again != got {
			t.Fatalf("canonical roundtrip=(%v,%v), want %v", again, err, got)
		}
		second, err := again.MarshalJSON()
		if err != nil || !bytes.Equal(second, canonical) {
			t.Fatalf("second canonical=(%s,%v), want %s", second, err, canonical)
		}
	})
}
