package chit

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
)

type entryNameCase struct {
	name  string
	value string
}

func TestEntryNamePortableBoundaryMatrix(t *testing.T) {
	t.Parallel()

	valid := []entryNameCase{
		{name: "single ordinary component", value: "result.json"},
		{name: "two ordinary components", value: "evidence/result.json"},
		{name: "three ordinary components", value: "a/b/c.txt"},
		{name: "spaces remain customer-visible", value: "space allowed/report 1.json"},
		{name: "unicode remains portable utf8", value: "unicode/證據.json"},
		{name: "component one below maximum", value: strings.Repeat("a", EntryNameComponentMaximumBytes-1)},
		{name: "component at maximum", value: strings.Repeat("a", EntryNameComponentMaximumBytes)},
		{name: "component maximum followed by one byte", value: strings.Repeat("a", EntryNameComponentMaximumBytes) + "/b"},
		{name: "component count one below maximum", value: strings.Repeat("a/", EntryNameMaximumComponents-2) + "z"},
		{name: "component count at maximum", value: strings.Repeat("a/", EntryNameMaximumComponents-1) + "z"},
		{name: "total extent one below maximum", value: entryNameAtExtent(t, EntryNameMaximumBytes-1)},
		{name: "total extent at maximum", value: entryNameAtExtent(t, EntryNameMaximumBytes)},
		{name: "punctuation has no hidden path meaning", value: "_/-/0"},
	}
	for _, tc := range valid {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := ParseEntryName(tc.value)
			if gotErr != nil || got.String() != tc.value || got.Validate() != nil {
				t.Fatalf("ParseEntryName(%q) = (%q, %v), want exact valid name", tc.value, got.String(), gotErr)
			}
			encoded, gotErr := got.MarshalJSON()
			if gotErr != nil {
				t.Fatalf("EntryName.MarshalJSON() error = %v, want nil", gotErr)
			}
			var roundTrip EntryName
			gotErr = roundTrip.UnmarshalJSON(encoded)
			if gotErr != nil || roundTrip != got {
				t.Fatalf("EntryName.UnmarshalJSON() = (%v, %v), want (%v, nil)", roundTrip, gotErr, got)
			}
		})
	}

	invalidUTF8 := string([]byte{utf8.RuneSelf})
	invalid := []entryNameCase{
		{name: "empty name", value: ""},
		{name: "leading separator creates empty component", value: "/leading"},
		{name: "trailing separator creates empty component", value: "trailing/"},
		{name: "double separator creates empty component", value: "double//separator"},
		{name: "current-directory component alone", value: "."},
		{name: "parent-directory component alone", value: ".."},
		{name: "nested current-directory component", value: "a/./b"},
		{name: "nested parent-directory component", value: "a/../b"},
		{name: "windows separator is platform-dependent", value: `windows\path`},
		{name: "nul byte cannot be displayed safely", value: "nul\x00path"},
		{name: "invalid utf8 is rejected", value: invalidUTF8},
		{name: "component one above maximum", value: strings.Repeat("a", EntryNameComponentMaximumBytes+1)},
		{name: "component count one above maximum", value: strings.Repeat("a/", EntryNameMaximumComponents) + "z"},
		{name: "total extent one above maximum", value: entryNameAtExtent(t, EntryNameMaximumBytes+1)},
	}
	preserved, gotErr := ParseEntryName("preserved.json")
	if gotErr != nil {
		t.Fatalf("ParseEntryName(preserved setup) error = %v, want nil", gotErr)
	}
	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := ParseEntryName(tc.value)
			if !errors.Is(gotErr, core.ErrChitContract) || got != (EntryName{}) {
				t.Fatalf("ParseEntryName(%q) = (%v, %v), want zero and errors.Is %v", tc.value, got, gotErr, core.ErrChitContract)
			}
			encoded, encodeErr := core.MarshalCanonicalJSONString(tc.value)
			if encodeErr != nil {
				return
			}
			got = preserved
			gotErr = got.UnmarshalJSON(encoded)
			if !errors.Is(gotErr, core.ErrJSONContract) || got != preserved {
				t.Fatalf("EntryName.UnmarshalJSON() = (%v, %v), want preserved receiver and errors.Is %v", got, gotErr, core.ErrJSONContract)
			}
		})
	}
}

func TestEntryNameJSONHostileFramingPreservesReceiver(t *testing.T) {
	t.Parallel()

	preserved, gotErr := ParseEntryName("preserved.json")
	if gotErr != nil {
		t.Fatalf("ParseEntryName(preserved setup) error = %v, want nil", gotErr)
	}
	valid, gotErr := preserved.MarshalJSON()
	if gotErr != nil {
		t.Fatalf("EntryName.MarshalJSON() setup error = %v, want nil", gotErr)
	}
	cases := []struct {
		name string
		data []byte
	}{
		{name: "empty document", data: nil},
		{name: "whitespace-only document", data: []byte(" \n\t")},
		{name: "null document", data: []byte("null")},
		{name: "array instead of string", data: []byte("[]")},
		{name: "object instead of string", data: []byte("{}")},
		{name: "number instead of string", data: []byte("1")},
		{name: "boolean instead of string", data: []byte("true")},
		{name: "truncated string", data: []byte(`"truncated`)},
		{name: "two concatenated strings", data: append(append([]byte(nil), valid...), valid...)},
		{name: "oversized string token", data: append(append([]byte{'"'}, bytes.Repeat([]byte{'a'}, EntryNameMaximumBytes+1)...), '"')},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := preserved
			gotErr := got.UnmarshalJSON(tc.data)
			if !errors.Is(gotErr, core.ErrJSONContract) || got != preserved {
				t.Fatalf("EntryName.UnmarshalJSON(%q) = (%v, %v), want preserved receiver and errors.Is %v", tc.data, got, gotErr, core.ErrJSONContract)
			}
		})
	}
}

func entryNameAtExtent(t *testing.T, wantBytes int) string {
	t.Helper()

	const componentCount = 18
	components := make([]string, componentCount)
	remaining := wantBytes - (componentCount - 1)
	for index := range components {
		remainingComponents := componentCount - index - 1
		componentBytes := min(EntryNameComponentMaximumBytes, remaining-remainingComponents)
		if componentBytes <= 0 {
			t.Fatalf("entry-name extent %d cannot form %d nonempty components", wantBytes, componentCount)
		}
		components[index] = strings.Repeat("a", componentBytes)
		remaining -= componentBytes
	}
	got := strings.Join(components, entryNameSeparator)
	if len(got) != wantBytes || remaining != 0 || len(components) > EntryNameMaximumComponents {
		t.Fatalf("entry-name fixture = (%d bytes, %d components), want (%d, at most %d)", len(got), len(components), wantBytes, EntryNameMaximumComponents)
	}
	return got
}
