package plunk

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func templateFixture(t testing.TB, kind TemplateKind) []byte {
	t.Helper()
	value := TemplateMetadata{ID: "template-fixture", Kind: kind}
	if err := value.Validate(); err != nil {
		t.Fatal(err)
	}
	wire, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return wire
}

func TestTemplateMetadataProjectionLayerTriad(t *testing.T) {
	t.Parallel()
	for _, kind := range []TemplateKind{TemplateMarketing, TemplateTransactional, TemplateHeadless} {
		wire := templateFixture(t, kind)
		got, err := DecodeTemplateMetadata(t.Context(), wire, core.ExtensibleJSONLimits())
		if err != nil || got != (TemplateMetadata{ID: "template-fixture", Kind: kind}) {
			t.Fatalf("provider template=%+v/%v,want exact kind and ID", got, err)
		}
	}
	// This is the documented full-template shape. The actual projection owns
	// only ID/type and must discard content, styling and future fields.
	type fullTemplate struct {
		ID        string       `json:"id"`
		Kind      TemplateKind `json:"type"`
		Name      string       `json:"name"`
		Body      string       `json:"body"`
		Subject   string       `json:"subject"`
		From      string       `json:"from"`
		ProjectID string       `json:"projectId"`
		Future    struct {
			Private string `json:"private"`
		} `json:"future"`
	}
	full := fullTemplate{ID: "template-fixture", Kind: TemplateMarketing, Name: "fixture", Body: strings.Repeat("private-provider-body", 1024), Subject: "subject", From: "sender@example.test", ProjectID: "project-fixture"}
	full.Future.Private = "private-provider-extension"
	wire, err := json.Marshal(full)
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecodeTemplateMetadata(t.Context(), wire, core.ExtensibleJSONLimits())
	if err != nil || got != (TemplateMetadata{ID: full.ID, Kind: full.Kind}) {
		t.Fatalf("full template projection=%+v/%v,want only ID/type", got, err)
	}
	canonical := templateFixture(t, TemplateMarketing)
	for _, tc := range []struct {
		name string
		wire []byte
	}{
		{"root null cannot invent metadata", []byte("null")},
		{"root array is not a template", []byte("[]")},
		{"missing facts cannot select template", []byte("{}")},
		{"unknown kind", bytes.Replace(canonical, []byte(`"MARKETING"`), []byte(`"PRIVATE-FUTURE-KIND"`), 1)},
		{"kind null", bytes.Replace(canonical, []byte(`"MARKETING"`), []byte(`null`), 1)},
		{"kind case differs", bytes.Replace(canonical, []byte(`"MARKETING"`), []byte(`"marketing"`), 1)},
		{"case-folded member", bytes.Replace(canonical, []byte(`"type"`), []byte(`"Type"`), 1)},
		{"empty id", bytes.Replace(canonical, []byte(`"template-fixture"`), []byte(`""`), 1)},
		{"wrong id kind", bytes.Replace(canonical, []byte(`"template-fixture"`), []byte(`17`), 1)},
		{"duplicate id", bytes.Replace(canonical, []byte(`"id":`), []byte(`"id":"private-duplicate","id":`), 1)},
		{"truncated", canonical[:len(canonical)-1]},
		{"second value", append(bytes.Clone(canonical), []byte("{}")...)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := DecodeTemplateMetadata(t.Context(), tc.wire, core.ExtensibleJSONLimits())
			if !errors.Is(err, core.ErrPlunkContract) || got != (TemplateMetadata{}) {
				t.Fatalf("template refusal=%+v/%v,want zero/contract", got, err)
			}
			if strings.Contains(err.Error(), "PRIVATE-FUTURE") || strings.Contains(err.Error(), "private-duplicate") {
				t.Fatal("provider value escaped safe diagnostic")
			}
		})
	}
}

func TestTemplateCallerLimitsAndContext(t *testing.T) {
	t.Parallel()
	wire := templateFixture(t, TemplateMarketing)
	for _, delta := range []int{-1, 0, 1} {
		limits := core.ExtensibleJSONLimits()
		var err error
		limits.DocumentMaximumBytes, err = core.NewByteCount(uint64(len(wire) + delta))
		if err != nil {
			t.Fatal(err)
		}
		got, err := DecodeTemplateMetadata(t.Context(), wire, limits)
		if delta < 0 {
			if !errors.Is(err, core.ErrPlunkContract) || got != (TemplateMetadata{}) {
				t.Fatal("caller byte limit did not refuse zero")
			}
		} else if err != nil || got.ID != "template-fixture" {
			t.Fatalf("caller exact byte boundary=%+v/%v", got, err)
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if got, err := DecodeTemplateMetadata(ctx, wire, core.ExtensibleJSONLimits()); !errors.Is(err, context.Canceled) || got != (TemplateMetadata{}) {
		t.Fatal("template cancellation lost native cause/zero")
	}
	var typedNil *sendNilContext
	if got, err := DecodeTemplateMetadata(typedNil, wire, core.ExtensibleJSONLimits()); !errors.Is(err, core.ErrContextObservation) || got != (TemplateMetadata{}) {
		t.Fatal("typed nil context escaped")
	}
}

func TestTemplateKindEveryByteAndNativeRefusal(t *testing.T) {
	t.Parallel()
	for raw := 0; raw <= math.MaxUint8; raw++ {
		kind := TemplateKind(raw)
		want := raw >= int(TemplateMarketing) && raw <= int(TemplateHeadless)
		if (kind.Validate() == nil) != want {
			t.Fatalf("kind%d admission wrong", raw)
		}
		encoded, err := kind.MarshalJSON()
		if !want {
			if !errors.Is(err, core.ErrPlunkContract) || encoded != nil {
				t.Fatal("invalid kind emitted bytes")
			}
			continue
		}
		var got TemplateKind
		if err := got.UnmarshalJSON(encoded); err != nil || got != kind {
			t.Fatalf("typed kind roundtrip=%v/%v", got, err)
		}
	}
	var absent *TemplateKind
	if err := absent.UnmarshalJSON([]byte("null")); !errors.Is(err, core.ErrPlunkContract) {
		t.Fatal("nil kind receiver admitted")
	}
}

func FuzzTemplateKindSemanticClosure(f *testing.F) {
	for _, kind := range []TemplateKind{TemplateMarketing, TemplateTransactional, TemplateHeadless} {
		wire, err := kind.MarshalJSON()
		if err != nil {
			f.Fatal(err)
		}
		f.Add(wire)
	}
	for _, raw := range []string{`null`, `"FUTURE"`, `1`, `"MARKETING" {}`} {
		f.Add([]byte(raw))
	}
	f.Fuzz(func(t *testing.T, raw []byte) {
		got := TemplateTransactional
		err := got.UnmarshalJSON(raw)
		var text string
		textErr := json.Unmarshal(raw, &text)
		want := TemplateKindInvalid
		if textErr == nil {
			switch text {
			case "MARKETING":
				want = TemplateMarketing
			case "TRANSACTIONAL":
				want = TemplateTransactional
			case "HEADLESS":
				want = TemplateHeadless
			}
		}
		if err != nil {
			if !errors.Is(err, core.ErrPlunkContract) || got != TemplateTransactional || want != TemplateKindInvalid {
				t.Fatal("kind refusal lost identity/receiver")
			}
			return
		}
		if got != want || got.Validate() != nil {
			t.Fatal("kind admitted wrong nominal value")
		}
		canonical, err := got.MarshalJSON()
		if err != nil {
			t.Fatal(err)
		}
		var round TemplateKind
		if err := round.UnmarshalJSON(canonical); err != nil || round != got {
			t.Fatal("kind canonical roundtrip changed")
		}
		second, err := round.MarshalJSON()
		if err != nil || !bytes.Equal(second, canonical) {
			t.Fatal("kind second write changed")
		}
	})
}

func FuzzTemplateMetadataProjection(f *testing.F) {
	for _, kind := range []TemplateKind{TemplateMarketing, TemplateTransactional, TemplateHeadless} {
		f.Add(templateFixture(f, kind))
	}
	for _, wire := range [][]byte{nil, []byte("null"), []byte("{}"), []byte(`{"id":"x","type":"FUTURE"}`)} {
		f.Add(wire)
	}
	f.Fuzz(func(t *testing.T, raw []byte) {
		got, err := DecodeTemplateMetadata(t.Context(), raw, core.ExtensibleJSONLimits())
		if err != nil {
			if !errors.Is(err, core.ErrPlunkContract) || got != (TemplateMetadata{}) {
				t.Fatal("projection refusal lost identity/zero")
			}
			return
		}
		if got.Validate() != nil {
			t.Fatal("invalid template projection escaped")
		}
		canonical, err := json.Marshal(got)
		if err != nil {
			t.Fatal(err)
		}
		round, err := DecodeTemplateMetadata(t.Context(), canonical, core.ExtensibleJSONLimits())
		if err != nil || round != got {
			t.Fatal("template canonical roundtrip changed facts")
		}
		second, err := json.Marshal(round)
		if err != nil || !bytes.Equal(canonical, second) {
			t.Fatal("template second canonical write changed")
		}
	})
}
