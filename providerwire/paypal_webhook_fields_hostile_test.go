package providerwire

import (
	"errors"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestPayPalWebhookFieldsMatchOfficialSchemaHostileBoundaryTable(t *testing.T) {
	t.Parallel()

	certificateAt := func(length int) string {
		const prefix = "https://api.paypal.com/v1/notifications/certs/"
		return prefix + strings.Repeat("a", length-len(prefix))
	}
	tests := []struct {
		name     string
		validate func() error
		wantErr  error
	}{
		{name: "auth algorithm documented SHA256withRSA is admitted", validate: func() error { return PayPalAuthAlgorithm("SHA256withRSA").Validate() }},
		{name: "auth algorithm exact one-byte minimum is admitted", validate: func() error { return PayPalAuthAlgorithm("A").Validate() }},
		{name: "auth algorithm exact 100-byte maximum is admitted", validate: func() error {
			return PayPalAuthAlgorithm(strings.Repeat("A", PayPalAuthAlgorithmMaximumBytes)).Validate()
		}},
		{name: "auth algorithm empty value is rejected", validate: func() error { return PayPalAuthAlgorithm("").Validate() }, wantErr: core.ErrProviderWireContract},
		{name: "auth algorithm punctuation is rejected", validate: func() error { return PayPalAuthAlgorithm("SHA256-with-RSA").Validate() }, wantErr: core.ErrProviderWireContract},
		{name: "auth algorithm one above maximum is rejected", validate: func() error {
			return PayPalAuthAlgorithm(strings.Repeat("A", PayPalAuthAlgorithmMaximumBytes+1)).Validate()
		}, wantErr: core.ErrProviderWireContract},
		{name: "certificate URI one below maximum is admitted", validate: func() error {
			return PayPalCertificateURL(certificateAt(PayPalCertificateURLMaximumBytes - 1)).Validate()
		}},
		{name: "certificate URI exact maximum is admitted", validate: func() error { return PayPalCertificateURL(certificateAt(PayPalCertificateURLMaximumBytes)).Validate() }},
		{name: "certificate URI one above maximum is rejected", validate: func() error {
			return PayPalCertificateURL(certificateAt(PayPalCertificateURLMaximumBytes + 1)).Validate()
		}, wantErr: core.ErrProviderWireContract},
		{name: "certificate relative reference is rejected", validate: func() error { return PayPalCertificateURL("/v1/notifications/certs/one").Validate() }, wantErr: core.ErrProviderWireContract},
		{name: "transmission identity ordinary provider UUID is admitted", validate: func() error { return PayPalTransmissionID("db49fb10-1343-11ef-ac58-e32457403f67").Validate() }},
		{name: "transmission identity exact two-byte minimum is admitted", validate: func() error { return PayPalTransmissionID("a1").Validate() }},
		{name: "transmission identity exact maximum is admitted", validate: func() error {
			return PayPalTransmissionID("a" + strings.Repeat("-", PayPalTransmissionIDMaximumBytes-1)).Validate()
		}},
		{name: "transmission identity one-byte value cannot satisfy both regex terms", validate: func() error { return PayPalTransmissionID("a").Validate() }, wantErr: core.ErrProviderWireContract},
		{name: "transmission identity all-decimal value is rejected", validate: func() error { return PayPalTransmissionID("12").Validate() }, wantErr: core.ErrProviderWireContract},
		{name: "transmission identity leading punctuation is rejected", validate: func() error { return PayPalTransmissionID("-a").Validate() }, wantErr: core.ErrProviderWireContract},
		{name: "transmission identity embedded whitespace is rejected", validate: func() error { return PayPalTransmissionID("a b").Validate() }, wantErr: core.ErrProviderWireContract},
		{name: "transmission identity one above maximum is rejected", validate: func() error {
			return PayPalTransmissionID("a" + strings.Repeat("-", PayPalTransmissionIDMaximumBytes)).Validate()
		}, wantErr: core.ErrProviderWireContract},
		{name: "transmission signature documented base64 shape is admitted", validate: func() error { return PayPalTransmissionSignature("ab2tJk1VCFm4EqdSuKqezr38rTdY3JeRQ==").Validate() }},
		{name: "transmission signature exact maximum is admitted", validate: func() error {
			return PayPalTransmissionSignature("a" + strings.Repeat("+", PayPalTransmissionSignatureMaximumBytes-1)).Validate()
		}},
		{name: "transmission signature one above maximum is rejected", validate: func() error {
			return PayPalTransmissionSignature("a" + strings.Repeat("+", PayPalTransmissionSignatureMaximumBytes)).Validate()
		}, wantErr: core.ErrProviderWireContract},
		{name: "transmission signature all-decimal value is rejected", validate: func() error { return PayPalTransmissionSignature("1234").Validate() }, wantErr: core.ErrProviderWireContract},
		{name: "transmission time UTC seconds is admitted", validate: func() error { return PayPalTransmissionTime("2026-08-31T20:00:00Z").Validate() }},
		{name: "transmission time fractional seconds is admitted", validate: func() error { return PayPalTransmissionTime("2026-08-31T20:00:00.123456789Z").Validate() }},
		{name: "transmission time explicit offset is admitted", validate: func() error { return PayPalTransmissionTime("2026-08-31T16:00:00-04:00").Validate() }},
		{name: "transmission time malformed calendar value is rejected", validate: func() error { return PayPalTransmissionTime("2026-02-30T20:00:00Z").Validate() }, wantErr: core.ErrProviderWireContract},
		{name: "transmission time one above published extent is rejected before parsing", validate: func() error {
			return PayPalTransmissionTime(strings.Repeat("1", PayPalTransmissionTimeMaximumBytes+1)).Validate()
		}, wantErr: core.ErrProviderWireContract},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			gotErr := testCase.validate()
			if !errors.Is(gotErr, testCase.wantErr) {
				t.Fatalf("PayPal documented field validation error = %v, want %v", gotErr, testCase.wantErr)
			}
		})
	}
}
