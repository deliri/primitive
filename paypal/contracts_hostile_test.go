package paypal

import (
	"errors"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestAccessTokenHostileCustodyBoundaries(t *testing.T) {
	t.Parallel()
	for _, testCase := range []struct {
		wantErr error
		name    string
		value   string
	}{
		{name: "one byte opaque token", value: "a"},
		{name: "exact custody maximum", value: strings.Repeat("a", core.PayPalAccessTokenCustodyMaximumBytes)},
		{name: "empty rejected", wantErr: core.ErrPayPalContract},
		{name: "space rejected by bearer grammar", value: "a b", wantErr: core.ErrPayPalContract},
		{name: "one above custody maximum", value: strings.Repeat("a", core.PayPalAccessTokenCustodyMaximumBytes+1), wantErr: core.ErrPayPalContract},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			token, err := ParseAccessToken([]byte(testCase.value))
			if testCase.wantErr != nil {
				if !errors.Is(err, testCase.wantErr) || token.Validate() == nil {
					t.Fatalf("ParseAccessToken() = (%v, %v), want rejected with %v", token, err, testCase.wantErr)
				}
				return
			}
			if err != nil || token.Validate() != nil {
				t.Fatalf("ParseAccessToken() error = %v validation = %v, want nil", err, token.Validate())
			}
			_ = token.Close()
		})
	}
}

func TestPublishedWebhookFieldLimits(t *testing.T) {
	t.Parallel()
	tests := []struct {
		parse   func(string) error
		name    string
		maximum int
	}{
		{name: "webhook id", maximum: core.PayPalWebhookIDMaximumBytes, parse: func(value string) error { _, err := ParsePayPalWebhookID(value); return err }},
		{name: "auth algorithm", maximum: core.PayPalAuthAlgorithmMaximumBytes, parse: func(value string) error { _, err := ParsePayPalAuthAlgorithm(value); return err }},
		{name: "transmission id", maximum: core.PayPalTransmissionIDMaximumBytes, parse: func(value string) error { _, err := ParsePayPalTransmissionID(value); return err }},
		{name: "transmission signature", maximum: core.PayPalTransmissionSignatureMaximumBytes, parse: func(value string) error { _, err := ParsePayPalTransmissionSignature(value); return err }},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			value := "A" + strings.Repeat("x", testCase.maximum-1)
			if err := testCase.parse(value); err != nil {
				t.Fatalf("parse(exact published maximum) error = %v, want nil", err)
			}
			if err := testCase.parse(value + "x"); !errors.Is(err, core.ErrPayPalContract) {
				t.Fatalf("parse(one above published maximum) error = %v, want %v", err, core.ErrPayPalContract)
			}
		})
	}
}

func TestPublishedRequestIDLimitRemainsNominal(t *testing.T) {
	t.Parallel()
	if core.PayPalRequestIDMaximumBytes != 38 {
		t.Fatalf("core.PayPalRequestIDMaximumBytes = %d, want PayPal-published 38", core.PayPalRequestIDMaximumBytes)
	}
}
