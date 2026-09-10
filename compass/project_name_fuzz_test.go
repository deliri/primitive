package compass_test

import (
	"bytes"
	json "encoding/json/v2"
	"errors"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/compass"
	"github.com/deliri/primitive/v2026/core"
)

func admittedProjectName(value string) bool {
	return value != "" && utf8.ValidString(value) && strings.TrimSpace(value) == value && strings.IndexFunc(value, unicode.IsControl) < 0
}
func FuzzProjectNameTextAndJSONSemanticClosure(f *testing.F) {
	for _, text := range []string{"Project", strings.Repeat("n", 129), strings.Repeat("界", 4096)} {
		name, err := compass.ParseProjectName(text)
		if err != nil {
			f.Fatal(err)
		}
		data, err := name.MarshalJSON()
		if err != nil {
			f.Fatal(err)
		}
		f.Add(text, data)
	}
	f.Add("", []byte("null"))
	f.Add("\xff", []byte("\"\\ud800\""))
	f.Add("A\nB", []byte("\"A\\nB\""))
	f.Fuzz(func(t *testing.T, text string, data []byte) {
		got, err := compass.ParseProjectName(text)
		wantOK := admittedProjectName(text)
		if (err == nil) != wantOK {
			t.Fatalf("name admitted=%t error=%v, want %t", err == nil, err, wantOK)
		}
		if wantOK {
			if got.String() != text || got.Validate() != nil {
				t.Fatalf("name got=%q, want exact %q", got.String(), text)
			}
		} else if got != (compass.ProjectName{}) || !errors.Is(err, core.ErrCompassContract) {
			t.Fatalf("name got=%v error=%v, want zero typed rejection", got, err)
		}
		before, err := compass.ParseProjectName("preserved")
		if err != nil {
			t.Fatal(err)
		}
		receiver := before
		var reference string
		referenceErr := json.Unmarshal(data, &reference)
		wantJSON := referenceErr == nil && admittedProjectName(reference)
		err = receiver.UnmarshalJSON(data)
		if (err == nil) != wantJSON {
			t.Fatalf("JSON name admitted=%t error=%v, want %t", err == nil, err, wantJSON)
		}
		if !wantJSON {
			if receiver != before || !errors.Is(err, core.ErrJSONContract) || !errors.Is(err, core.ErrCompassContract) {
				t.Fatalf("rejected JSON name changed receiver=%t error=%v", receiver != before, err)
			}
			return
		}
		if receiver.String() != reference || receiver.Validate() != nil {
			t.Fatalf("JSON name got=%q, want %q", receiver.String(), reference)
		}
		encoded, err := receiver.MarshalJSON()
		if err != nil {
			t.Fatal(err)
		}
		var roundTrip compass.ProjectName
		if err := roundTrip.UnmarshalJSON(encoded); err != nil || roundTrip != receiver {
			t.Fatalf("JSON round trip=%v error=%v, want %v", roundTrip, err, receiver)
		}
		canonicalAgain, err := roundTrip.MarshalJSON()
		if err != nil || !bytes.Equal(canonicalAgain, encoded) {
			t.Fatalf("second canonical bytes=%q error=%v, want %q", canonicalAgain, err, encoded)
		}

	})
}

func TestProjectNameJSONReceiverLayerTriad(t *testing.T) {
	t.Parallel()
	before, err := compass.ParseProjectName("preserved")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name        string
		data        []byte
		nilReceiver bool
		wantText    string
		wantErr     error
	}{
		{name: "new_name", data: []byte("\"new\""), wantText: "new"},
		{name: "escaped_quote", data: []byte("\"A\\\"B\""), wantText: "A\"B"},
		{name: "outer_whitespace", data: []byte(" \n\"new\"\t"), wantText: "new"},
		{name: "null", data: []byte("null"), wantErr: core.ErrJSONContract},
		{name: "empty_name", data: []byte("\"\""), wantErr: core.ErrJSONContract},
		{name: "numeric_value", data: []byte("1"), wantErr: core.ErrJSONContract},
		{name: "control_escape", data: []byte("\"A\\nB\""), wantErr: core.ErrJSONContract},
		{name: "unpaired_surrogate", data: []byte("\"\\ud800\""), wantErr: core.ErrJSONContract},
		{name: "trailing_document", data: []byte("\"new\" {}"), wantErr: core.ErrJSONContract},
		{name: "neutral_absent_receiver", data: []byte("\"new\""), nilReceiver: true, wantErr: core.ErrJSONContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := before
			receiver := &got
			if tc.nilReceiver {
				receiver = nil
			}
			err := receiver.UnmarshalJSON(tc.data)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("got error=%v, want %v", err, tc.wantErr)
			}
			if tc.wantErr != nil {
				if got != before || !errors.Is(err, core.ErrCompassContract) {
					t.Fatalf("rejected name changed=%t error=%v, want preserved Compass refusal", got != before, err)
				}
				return
			}
			if got.String() != tc.wantText {
				t.Fatalf("name got=%q, want %q", got.String(), tc.wantText)
			}
		})
	}
}
