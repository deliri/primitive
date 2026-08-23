package objectstore

import (
	"encoding"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

const capabilityHeaderSecret = "TOPSECRETSIGNEDHEADERMATERIAL"

// TestObjectstoreBearerValuesRedactAtEveryReachableLayer is the disclosure
// ratchet for both valid and hostile values. It covers every public bearer
// layer a caller can reach, value and pointer method sets, zero values, every
// published transfer provider, both directions, an exact maximum header set,
// and the signed-header aggregate ceiling. A formatter regression therefore
// cannot hide behind an outer type that still redacts.
func TestObjectstoreBearerValuesRedactAtEveryReachableLayer(t *testing.T) {
	t.Parallel()

	ordinaryHeader := secretSignedHeaderFixture(t, "X-Goog-Meta-Run", capabilityHeaderSecret)
	ordinaryHeaders, ordinaryHeadersErr := NewSignedHeaders([]SignedHeader{ordinaryHeader})
	if ordinaryHeadersErr != nil {
		t.Fatalf("NewSignedHeaders(ordinary) setup error = %v, want nil", ordinaryHeadersErr)
	}

	secretURL, secretURLErr := ParseSignedURL(capabilityGCSMetadataRunURL)
	if secretURLErr != nil {
		t.Fatalf("ParseSignedURL(secret target) setup error = %v, want nil", secretURLErr)
	}
	secretUpload := UploadTarget{
		Headers: ordinaryHeaders, URL: secretURL, ExpiresAt: providerFutureInstant(t),
	}
	if err := secretUpload.validateFor(ProviderGoogleCloudStorage); err != nil {
		t.Fatalf("UploadTarget.validateFor(secret target) setup error = %v, want nil", err)
	}
	googleUpload := providerUploadTarget(t, ProviderGoogleCloudStorage)
	amazonUpload := providerUploadTarget(t, ProviderAmazonS3)
	cloudflareUpload := providerUploadTarget(t, ProviderCloudflareImages)
	googleDownload := downloadTargetFixture(t, downloadCapabilityFixtureRequest{Provider: ProviderGoogleCloudStorage})
	amazonDownload := downloadTargetFixture(t, downloadCapabilityFixtureRequest{Provider: ProviderAmazonS3})

	values := []struct {
		value fmt.Formatter
		name  string
	}{
		{name: "zero signed URL", value: SignedURL{}},
		{name: "valid signed URL", value: googleUpload.URL},
		{name: "zero signed header", value: SignedHeader{}},
		{name: "ordinary signed header", value: ordinaryHeader},
		{name: "zero signed headers", value: SignedHeaders{}},
		{name: "ordinary signed headers", value: ordinaryHeaders},
		{name: "zero upload target", value: UploadTarget{}},
		{name: "google upload target", value: googleUpload},
		{name: "google secret-header upload target", value: secretUpload},
		{name: "amazon upload target", value: amazonUpload},
		{name: "cloudflare upload target", value: cloudflareUpload},
		{name: "zero download target", value: DownloadTarget{}},
		{name: "google download target", value: googleDownload},
		{name: "amazon download target", value: amazonDownload},
		{name: "pointer to signed header", value: &ordinaryHeader},
		{name: "pointer to signed headers", value: &ordinaryHeaders},
		{name: "pointer to upload target", value: &secretUpload},
		{name: "pointer to download target", value: &googleDownload},
	}
	formats := []struct {
		name      string
		pattern   string
		wantExact bool
	}{
		{name: "default value", pattern: "%v", wantExact: true},
		{name: "field value", pattern: "%+v", wantExact: true},
		{name: "Go syntax", pattern: "%#v", wantExact: true},
		{name: "string", pattern: "%s", wantExact: true},
		{name: "quoted string", pattern: "%q", wantExact: true},
		{name: "decimal", pattern: "%d", wantExact: true},
		{name: "binary", pattern: "%b", wantExact: true},
		{name: "character", pattern: "%c", wantExact: true},
		{name: "lower hexadecimal", pattern: "%x", wantExact: true},
		{name: "upper hexadecimal", pattern: "%X", wantExact: true},
		{name: "octal", pattern: "%o", wantExact: true},
		{name: "prefixed octal", pattern: "%O", wantExact: true},
		{name: "Unicode", pattern: "%U", wantExact: true},
		{name: "boolean", pattern: "%t", wantExact: true},
		{name: "lower exponent", pattern: "%e", wantExact: true},
		{name: "upper exponent", pattern: "%E", wantExact: true},
		{name: "lower decimal point", pattern: "%f", wantExact: true},
		{name: "upper decimal point", pattern: "%F", wantExact: true},
		{name: "compact lower exponent", pattern: "%g", wantExact: true},
		{name: "compact upper exponent", pattern: "%G", wantExact: true},
		{name: "left width", pattern: "%-20v", wantExact: true},
		{name: "zero width", pattern: "%020v", wantExact: true},
		{name: "precision", pattern: "%.3v", wantExact: true},
		{name: "space flag", pattern: "% v", wantExact: true},
		{name: "dynamic type", pattern: "%T"},
		{name: "pointer identity", pattern: "%p"},
	}
	for _, value := range values {
		t.Run(value.name, func(t *testing.T) {
			t.Parallel()

			for _, format := range formats {
				t.Run(format.name, func(t *testing.T) {
					t.Parallel()

					rendered := fmt.Sprintf(format.pattern, value.value)
					if format.wantExact && rendered != core.RedactedValueText {
						t.Fatalf("fmt.Sprintf(%q) = %q, want %q", format.pattern, rendered, core.RedactedValueText)
					}
					if strings.Contains(rendered, capabilityHeaderSecret) ||
						strings.Contains(rendered, capabilitySecret) ||
						strings.Contains(rendered, core.SchemeHTTPS) {
						t.Fatalf("fmt.Sprintf(%q) disclosed bearer material", format.pattern)
					}
				})
			}
		})
	}
}

// TestObjectstoreNestedBearerValuesHaveNoImplicitPersistenceProjection proves
// the four newly closed values cannot acquire a text or explicit JSON encoder.
// The only bearer disclosure boundary remains the nominal issue-only
// capability projection; JSON v2 refuses structs whose only state is private
// rather than silently projecting an empty document.
func TestObjectstoreNestedBearerValuesHaveNoImplicitPersistenceProjection(t *testing.T) {
	t.Parallel()

	header := secretSignedHeaderFixture(t, "X-Goog-Meta-Run", capabilityHeaderSecret)
	headers, headersErr := NewSignedHeaders([]SignedHeader{header})
	if headersErr != nil {
		t.Fatalf("NewSignedHeaders() setup error = %v, want nil", headersErr)
	}
	values := []struct {
		value fmt.Formatter
		name  string
	}{
		{name: "signed header", value: header},
		{name: "signed headers", value: headers},
		{name: "upload target", value: UploadTarget{Headers: headers, URL: providerSignedURL(t, ProviderGoogleCloudStorage, DirectionUpload), ExpiresAt: providerFutureInstant(t)}},
		{name: "download target", value: DownloadTarget{Headers: headers, URL: providerSignedURL(t, ProviderGoogleCloudStorage, DirectionDownload), ExpiresAt: providerFutureInstant(t)}},
	}
	for _, value := range values {
		t.Run(value.name, func(t *testing.T) {
			t.Parallel()

			if _, ok := value.value.(encoding.TextMarshaler); ok {
				t.Fatalf("%s implements encoding.TextMarshaler, want no text disclosure boundary", value.name)
			}
			if _, ok := value.value.(json.Marshaler); ok {
				t.Fatalf("%s implements json.Marshaler, want no JSON disclosure boundary", value.name)
			}
			encoded, err := json.Marshal(value.value)
			if _, ok := errors.AsType[*json.SemanticError](err); !ok {
				t.Fatalf("json.Marshal(%s) = (%q, %v), want *json.SemanticError", value.name, encoded, err)
			}
			if strings.Contains(string(encoded), capabilityHeaderSecret) ||
				strings.Contains(string(encoded), capabilitySecret) ||
				strings.Contains(string(encoded), core.SchemeHTTPS) {
				t.Fatalf("json.Marshal(%s) = %q, want no bearer material", value.name, encoded)
			}
		})
	}
}

func secretSignedHeaderFixture(t testing.TB, name, value string) SignedHeader {
	t.Helper()

	typedName, nameErr := core.ParseHTTPHeaderName(name)
	if nameErr != nil {
		t.Fatalf("core.ParseHTTPHeaderName() setup error = %v, want nil", nameErr)
	}
	header, headerErr := NewSignedHeader(typedName, value)
	if headerErr != nil {
		t.Fatalf("NewSignedHeader() setup error = %v, want nil", headerErr)
	}
	return header
}
