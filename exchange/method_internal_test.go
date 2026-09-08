package exchange

import (
	"bytes"
	json "encoding/json/v2"
	"errors"
	"math"
	"net/http"
	"strconv"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestMethodExhaustsClosedDomain(t *testing.T) {
	t.Parallel()
	bindings := []struct {
		method Method
		wire   string
	}{
		{MethodGet, http.MethodGet}, {MethodHead, http.MethodHead}, {MethodPost, http.MethodPost},
		{MethodPut, http.MethodPut}, {MethodPatch, http.MethodPatch}, {MethodDelete, http.MethodDelete}, {MethodOptions, http.MethodOptions},
	}
	cases := make([]struct {
		name     string
		input    Method
		wantWire string
		wantErr  error
	}, 0, math.MaxUint8+1)
	for raw := range math.MaxUint8 + 1 {
		row := struct {
			name     string
			input    Method
			wantWire string
			wantErr  error
		}{name: strconv.Itoa(raw), input: Method(raw), wantErr: core.ErrExchangeContract}
		for _, binding := range bindings {
			if row.input == binding.method {
				row.wantWire, row.wantErr = binding.wire, nil
			}
		}
		cases = append(cases, row)
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotErr := tc.input.Validate()
			gotWire := tc.input.String()
			gotValid := tc.input.IsValid()
			encoded, encodeErr := tc.input.MarshalJSON()
			if !errors.Is(gotErr, tc.wantErr) || gotWire != tc.wantWire || gotValid != (tc.wantErr == nil) {
				t.Fatalf("method %d=(%q,%t,%v), want (%q,%t,%v)", uint8(tc.input), gotWire, gotValid, gotErr, tc.wantWire, tc.wantErr == nil, tc.wantErr)
			}
			if tc.wantErr != nil {
				if encoded != nil || !errors.Is(encodeErr, core.ErrJSONContract) || !errors.Is(encodeErr, core.ErrExchangeContract) {
					t.Fatalf("invalid method emitted (%q,%v), want absent encoding and both identities", encoded, encodeErr)
				}
				return
			}
			wantJSON, err := json.Marshal(tc.wantWire)
			if err != nil {
				t.Fatal(err)
			}
			if encodeErr != nil || !bytes.Equal(encoded, wantJSON) {
				t.Fatalf("method JSON=(%q,%v), want Go token encoding %q", encoded, encodeErr, wantJSON)
			}
			parsed, err := parseMethod(tc.wantWire)
			if err != nil || parsed != tc.input {
				t.Fatalf("Go token parsed=(%v,%v), want exact nominal %v", parsed, err, tc.input)
			}
			receiver := MethodUnknown
			if err := receiver.UnmarshalJSON(encoded); err != nil || receiver != tc.input {
				t.Fatalf("method decode=(%v,%v), want exact %v", receiver, err, tc.input)
			}
		})
	}
}

func TestMethodJSONReceiverLayerTriad(t *testing.T) {
	t.Parallel()
	canonical, err := json.Marshal(http.MethodGet)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name        string
		wire        []byte
		nilReceiver bool
		want        Method
		wantErr     error
	}{
		{name: "canonical method replaces populated receiver", wire: canonical, want: MethodGet},
		{name: "Go-permitted surrounding whitespace is neutral", wire: append(append([]byte{' ', '\t'}, canonical...), '\n'), want: MethodGet},
		{name: "nil receiver cannot accept canonical method", wire: canonical, nilReceiver: true, wantErr: core.ErrJSONContract},
		{name: "lowercase cannot silently normalize a method", wire: []byte(`"get"`), want: MethodPatch, wantErr: core.ErrJSONContract},
		{name: "unknown method cannot acquire a supported nominal arm", wire: []byte(`"FUTURE"`), want: MethodPatch, wantErr: core.ErrJSONContract},
		{name: "null cannot erase a populated receiver", wire: []byte("null"), want: MethodPatch, wantErr: core.ErrJSONContract},
		{name: "truncated token cannot publish a partial method", wire: canonical[:len(canonical)-1], want: MethodPatch, wantErr: core.ErrJSONContract},
		{name: "trailing value cannot hide behind valid first token", wire: append(bytes.Clone(canonical), []byte(" null")...), want: MethodPatch, wantErr: core.ErrJSONContract},
		{name: "absent token cannot erase a populated receiver", want: MethodPatch, wantErr: core.ErrJSONContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			value := MethodPatch
			receiver := &value
			if tc.nilReceiver {
				receiver = nil
			}
			err := receiver.UnmarshalJSON(tc.wire)
			if !errors.Is(err, tc.wantErr) || tc.wantErr != nil && !errors.Is(err, core.ErrExchangeContract) {
				t.Fatalf("method decode error=%v, want JSON/Exchange refusal %v", err, tc.wantErr)
			}
			if !tc.nilReceiver && value != tc.want {
				t.Fatalf("receiver=%v, want %v", value, tc.want)
			}
			if tc.nilReceiver && value != MethodPatch {
				t.Fatalf("populated method=%v, want %v", value, MethodPatch)
			}
		})
	}
}
