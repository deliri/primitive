package exchange

import (
	"bytes"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

const capturedValueFixture = "opaque-header-value"

// This is a direct helper ratchet: conversion must enforce the owning Header
// count limit before constructing any typed values, even though its caller also
// validates the completed field. Arbitrary value grammar remains Core's rule.
func TestHeaderValueConversionBoundaries(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		count       int
		replacement string
		replaceAt   int
		replace     bool
		wantCount   int
		wantErr     error
	}{
		{name: "neutral no values produces no invented field value"},
		{name: "positive one value cannot disappear", count: 1, wantCount: 1},
		{name: "boundary one below value ceiling retains ordering", count: HeaderValueMaximumCount - 1, wantCount: HeaderValueMaximumCount - 1},
		{name: "boundary exact value ceiling is not refused", count: HeaderValueMaximumCount, wantCount: HeaderValueMaximumCount},
		{name: "negative one above ceiling is refused before projection", count: HeaderValueMaximumCount + 1, wantErr: core.ErrExchangeContract},
		{name: "boundary exact value byte ceiling is admitted", count: 1, replace: true, replacement: strings.Repeat("a", HeaderValueMaximumBytes), wantCount: 1},
		{name: "negative one above value byte ceiling returns no partial values", count: 2, replace: true, replaceAt: 1, replacement: strings.Repeat("a", HeaderValueMaximumBytes+1), wantErr: core.ErrExchangeContract},
		{name: "negative injected final value cannot leak valid prefix", count: HeaderValueMaximumCount, replace: true, replaceAt: HeaderValueMaximumCount - 1, replacement: "\r\n", wantErr: core.ErrExchangeContract},
		{name: "positive empty wire value remains a present valid value", count: 1, replace: true, wantCount: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			wire := captureFixtureValues(tc.count)
			if tc.replace {
				wire[tc.replaceAt] = tc.replacement
			}
			got, gotErr := headerValues(wire)
			if !errors.Is(gotErr, tc.wantErr) || len(got) != tc.wantCount {
				t.Fatalf("headerValues() count/error = (%d, %v), want (%d, %v)", len(got), gotErr, tc.wantCount, tc.wantErr)
			}
			if tc.wantErr != nil && got != nil {
				t.Fatalf("refused values = %v, want nil", got)
			}
			for i, value := range got {
				gotWire, err := value.Value()
				if err != nil || gotWire != wire[i] {
					t.Fatalf("value[%d] = (%q, %v), want (%q, nil)", i, gotWire, err, wire[i])
				}
				wantWire := wire[i]
				wire[i] = "mutated-source"
				gotWire, err = value.Value()
				if err != nil || gotWire != wantWire {
					t.Fatalf("value[%d] borrowed mutable source slot: (%q, %v), want (%q, nil)", i, gotWire, err, wantWire)
				}
			}
		})
	}
}

// Conversion and capture are distinct ownership boundaries. This table catches
// a valid prefix being returned beside a later field's refusal and confirms
// unselected hostile fields do not affect the requested projection.
func TestHeaderCaptureCustodyLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name               string
		count              int
		invalidLast        bool
		selectField        bool
		prefix             bool
		duplicateSelection bool
		wantSelectionErr   error
		wantFields         int
		wantValues         int
		wantErr            error
	}{
		{name: "positive exact selected ceiling retains every ordered value", count: HeaderValueMaximumCount, selectField: true, wantFields: 1, wantValues: HeaderValueMaximumCount},
		{name: "negative selected value ceiling cannot return invalid typed field", count: HeaderValueMaximumCount + 1, selectField: true, wantErr: core.ErrExchangeContract},
		{name: "negative later refused field withholds earlier captured field", count: HeaderValueMaximumCount + 1, selectField: true, prefix: true, wantErr: core.ErrExchangeContract},
		{name: "negative invalid final value withholds all prior projections", count: HeaderValueMaximumCount, selectField: true, invalidLast: true, prefix: true, wantErr: core.ErrExchangeContract},
		{name: "neutral selected but absent header does not invent an empty field", selectField: true},
		{name: "neutral unselected hostile values remain outside projection", count: HeaderValueMaximumCount + 1, invalidLast: true},
		{name: "negative duplicate selection cannot produce an invalid capture", count: 1, selectField: true, duplicateSelection: true, wantSelectionErr: core.ErrExchangeContract, wantErr: core.ErrExchangeContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			name := core.HTTPHeaderAccept()
			wire := captureFixtureValues(tc.count)
			if tc.invalidLast {
				wire[len(wire)-1] = "\r\n"
			}
			headers := make(http.Header)
			headers[name.String()] = wire
			selection := HeaderSelection{}
			if tc.prefix {
				prefix := core.HTTPHeaderHost()
				headers.Set(prefix.String(), capturedValueFixture)
				selection.Names = append(selection.Names, prefix)
			}
			if tc.selectField {
				selection.Names = append(selection.Names, name)
			}
			if tc.duplicateSelection {
				selection.Names = append(selection.Names, name)
			}
			if err := selection.Validate(); !errors.Is(err, tc.wantSelectionErr) {
				t.Fatalf("fixture selection = %v, want %v", err, tc.wantSelectionErr)
			}
			got, gotErr := captureHeaders(headers, selection)
			if !errors.Is(gotErr, tc.wantErr) || len(got.Values) != tc.wantFields {
				t.Fatalf("capture fields/error = (%d, %v), want (%d, %v)", len(got.Values), gotErr, tc.wantFields, tc.wantErr)
			}
			if tc.wantErr != nil && got.Values != nil {
				t.Fatalf("refused captured fields = %v, want nil", got.Values)
			}
			if err := got.Validate(); err != nil {
				t.Fatalf("capture result validation = %v, want nil", err)
			}
			for _, field := range got.Values {
				if field.Name != name || len(field.Values) != tc.wantValues {
					t.Fatalf("field name/count = (%v, %d), want (%v, %d)", field.Name, len(field.Values), name, tc.wantValues)
				}
				for i, value := range field.Values {
					gotWire, err := value.Value()
					if err != nil || gotWire != wire[i] {
						t.Fatalf("captured value[%d] = (%q, %v), want (%q, nil)", i, gotWire, err, wire[i])
					}
				}
			}
		})
	}
}

func captureFixtureValues(count int) []string {
	values := make([]string, count)
	for i := range values {
		values[i] = capturedValueFixture + strconv.Itoa(i)
	}
	return values
}

type captureFixtureBody struct {
	source bytes.Reader
	reads  int
	closes int
}

func (b *captureFixtureBody) Read(p []byte) (int, error) { b.reads++; return b.source.Read(p) }
func (b *captureFixtureBody) Close() error               { b.closes++; return nil }
