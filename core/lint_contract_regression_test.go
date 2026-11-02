package core

import (
	jsontext "encoding/json/jsontext"
	"errors"
	"math"
	"testing"
)

func TestSignatureV4QueryFieldExhaustsBackingDomain(t *testing.T) {
	t.Parallel()
	names := [...]string{"", SignatureV4AlgorithmQuery, SignatureV4CredentialQuery, SignatureV4DateQuery, SignatureV4SignedHeadersQuery, SignatureV4SignatureQuery}
	for raw := range 256 {
		field := SignatureV4QueryField(raw)
		want := raw > 0 && raw < len(names)
		var wantErr error
		if !want {
			wantErr = ErrExchangeContract
		}
		wantText := ""
		if want {
			wantText = names[raw]
		}
		if field.IsValid() != want || !errors.Is(field.Validate(), wantErr) || field.String() != wantText {
			t.Fatalf("query field byte%d valid/error/text = %t/%v/%q, want %t/%v/%q", raw, field.IsValid(), field.Validate(), field.String(), want, wantErr, wantText)
		}
	}
}

// The portable HTTP agreement is deliberately the native standard-library
// ceiling, rather than a second independently maintained numeric policy.
func TestHTTPServerHeaderCeilingTracksNativePortableInt(t *testing.T) {
	t.Parallel()
	if got := HTTPServerHeaderMaximumBytes; got != math.MaxInt32 {
		t.Fatalf("portable header ceiling = %d, want native %d", got, math.MaxInt32)
	}
}

func TestStrictJSONScannerTokenKindExhaustsNativeByteDomain(t *testing.T) {
	t.Parallel()
	for raw := range 256 {
		var scanner strictJSONScanState
		kind := jsontext.Kind(raw)
		wantAccepted := kind == jsontext.KindNull || kind == jsontext.KindFalse || kind == jsontext.KindTrue || kind == jsontext.KindString || kind == jsontext.KindNumber || kind == jsontext.KindBeginObject || kind == jsontext.KindBeginArray
		got := scanner.scanToken(kind, DefaultStrictJSONLimits())
		if (got == nil) != wantAccepted || got != nil && !errors.Is(got, ErrJSONContract) {
			t.Fatalf("native token kind byte%d error = %v, want accepted=%t with JSONContract refusal", raw, got, wantAccepted)
		}
	}
}
