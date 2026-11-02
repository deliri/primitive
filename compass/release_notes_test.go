package compass_test

import (
	"bytes"
	"encoding/json/jsontext"
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/compass"
	"github.com/deliri/primitive/v2026/core"
)

func TestConfigurationBindsReleaseNotesToAuthoredVersion(t *testing.T) {
	t.Parallel()
	for _, contract := range []struct {
		name   string
		notes  []compass.ReleaseNote
		refuse bool
	}{
		{name: "single change", notes: []compass.ReleaseNote{"Preserve native error causes."}},
		{name: "ordered changes", notes: []compass.ReleaseNote{"First change.", "Second change."}},
		{name: "unicode paragraphs and spacing", notes: []compass.ReleaseNote{"Café parsing.\n\nRetain causes.", "  Keep spacing.  "}},
		{name: "beyond native window", notes: []compass.ReleaseNote{compass.ReleaseNote(strings.Repeat("x", 65537))}},
		{name: "more than former array window", notes: func() []compass.ReleaseNote {
			v := make([]compass.ReleaseNote, 1025)
			for i := range v {
				v[i] = "Authored repeated change."
			}
			return v
		}()},
		{name: "absent notes refused", refuse: true},
		{name: "empty array refused", notes: []compass.ReleaseNote{}, refuse: true},
		{name: "empty entry refused", notes: []compass.ReleaseNote{""}, refuse: true},
		{name: "blank entry refused", notes: []compass.ReleaseNote{" \n\t\u00a0"}, refuse: true},
		{name: "blank among valid changes refused", notes: []compass.ReleaseNote{"Real change.", " ", "Another change."}, refuse: true},
	} {
		t.Run(contract.name, func(t *testing.T) {
			t.Parallel()
			project := projectFixture(t, "Release tool", "example.com/tool", "owner/tool", 2026, 1, 1)
			project.ReleaseNotes = contract.notes
			authored := compass.Configuration{Project: project}
			encoded, encodeErr := core.MarshalCanonicalJSONDocument(authored)
			if encodeErr != nil {
				t.Fatalf("encode authored notes=%v, want native JSON", encodeErr)
			}
			got, decodeErr := compass.Decode[compass.Configuration](bytes.NewReader(encoded))
			if contract.refuse {
				if !errors.Is(decodeErr, core.ErrCompassContract) || !sameConfiguration(got, compass.Configuration{}) {
					t.Fatalf("notes config=%+v error=%v, want zero and Compass refusal", got, decodeErr)
				}
				return
			}
			if decodeErr != nil || !sameConfiguration(got, authored) {
				t.Fatalf("config=%+v error=%v, want exact authored version and ordered notes=%+v", got, decodeErr, authored)
			}
		})
	}
}

func TestConfigurationRefusesMissingReleaseNotes(t *testing.T) {
	t.Parallel()
	encoded := encodedConfiguration(t, projectFixture(t, "Release tool", "example.com/tool", "owner/tool", 2026, 1, 1))
	member := []byte(",\"release_notes\":[\"Fixture release change.\"]")
	if bytes.Count(encoded, member) != 1 {
		t.Fatalf("authored fixture=%s, want exactly one notes member", encoded)
	}
	missing := bytes.Replace(encoded, member, nil, 1)
	got, decodeErr := compass.Decode[compass.Configuration](bytes.NewReader(missing))
	if !errors.Is(decodeErr, core.ErrCompassContract) || !sameConfiguration(got, compass.Configuration{}) {
		t.Errorf("missing notes config=%+v error=%v, want zero and Compass refusal", got, decodeErr)
	}
}

func TestReleaseNotesRefuseInvalidUTF8WithoutNormalizingText(t *testing.T) {
	t.Parallel()
	for _, text := range []string{"x", "  kept  ", "line one\nline two", "", " \n\t", "\xff"} {
		admitted := utf8.ValidString(text) && strings.TrimSpace(text) != ""
		note, noteErr := compass.ParseReleaseNote(text)
		if (noteErr == nil) != admitted {
			t.Fatalf("parse note=%q error=%v, want admitted=%t", text, noteErr, admitted)
		}
		if admitted {
			if note.String() != text || note.Validate() != nil {
				t.Errorf("admitted note=%q, want exact authored %q", note.String(), text)
			}
		} else if note != compass.ReleaseNote("") || !errors.Is(noteErr, core.ErrCompassContract) {
			t.Errorf("refused note=%q error=%v, want zero and Compass refusal", note, noteErr)
		}
		castErr := compass.ReleaseNote(text).Validate()
		if (castErr == nil) != admitted || !admitted && !errors.Is(castErr, core.ErrCompassContract) {
			t.Errorf("cast note=%q error=%v, want admitted=%t", text, castErr, admitted)
		}
	}
}

func TestConfigurationReleaseNotesWireRefusals(t *testing.T) {
	t.Parallel()
	valid := encodedConfiguration(t, projectFixture(t, "Release tool", "example.com/tool", "owner/tool", 2026, 1, 1))
	notes := []byte("[\"Fixture release change.\"]")
	if bytes.Count(valid, notes) != 1 {
		t.Fatalf("authored fixture=%s, want exactly one notes array", valid)
	}
	for _, token := range []jsontext.Value{jsontext.Value("null"), jsontext.Value("42"), jsontext.Value("{}"), jsontext.Value("true"), jsontext.Value("\"single string\"")} {
		encoded := bytes.Replace(valid, notes, token, 1)
		got, decodeErr := compass.Decode[compass.Configuration](bytes.NewReader(encoded))
		if !errors.Is(decodeErr, core.ErrCompassContract) || !sameConfiguration(got, compass.Configuration{}) {
			t.Errorf("notes token=%s config=%+v error=%v, want zero and Compass refusal", token, got, decodeErr)
		}
	}
	duplicate := append(bytes.Clone(valid[:len(valid)-2]), []byte(",\"release_notes\":[\"second\"]}}")...)
	got, decodeErr := compass.Decode[compass.Configuration](bytes.NewReader(duplicate))
	if !errors.Is(decodeErr, core.ErrCompassContract) || !sameConfiguration(got, compass.Configuration{}) {
		t.Errorf("duplicate notes config=%+v error=%v, want zero and Compass refusal", got, decodeErr)
	}
}
