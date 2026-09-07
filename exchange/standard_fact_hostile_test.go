package exchange

import (
	"errors"
	"fmt"
	"math"
	"mime"
	"net/http"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestStandardHeaderExhaustsCompleteByteDomain(t *testing.T) {
	t.Parallel()
	// This exhausts a byte-backed enum; the rejected ordinals are one domain
	// class, not 252 earned rows toward a parser or classifier quota. Expected
	// names bind to the owning constants, independently of the enum's lookup.
	type headerCase struct {
		name      string
		input     StandardHeader
		wantName  string
		wantValid bool
		wantErr   error
	}
	cases := [math.MaxUint8 + 1]headerCase{}
	for ordinal := range cases {
		cases[ordinal] = headerCase{name: fmt.Sprintf("unpublished header ordinal %d cannot acquire a field name", ordinal), input: StandardHeader(ordinal), wantErr: core.ErrExchangeContract}
	}
	cases[StandardHeaderAuthorization] = headerCase{name: "authorization cannot become another valid header", input: StandardHeaderAuthorization, wantName: authorizationHeaderNameText, wantValid: true}
	cases[StandardHeaderCacheControl] = headerCase{name: "cache policy cannot become another valid header", input: StandardHeaderCacheControl, wantName: cacheControlHeaderNameText, wantValid: true}
	cases[StandardHeaderForwardedFor] = headerCase{name: "forwarding identity cannot become another valid header", input: StandardHeaderForwardedFor, wantName: forwardedForHeaderNameText, wantValid: true}
	cases[StandardHeaderRetryAfter] = headerCase{name: "retry timing cannot become another valid header", input: StandardHeaderRetryAfter, wantName: retryAfterHeaderNameText, wantValid: true}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotErr := tc.input.Validate()
			gotName, gotNameErr := tc.input.Name()
			if !errors.Is(gotErr, tc.wantErr) || !errors.Is(gotNameErr, tc.wantErr) || tc.input.IsValid() != tc.wantValid {
				t.Fatalf("header validation/projection/validity = (%v, %v, %t), want (%v, %v, %t)", gotErr, gotNameErr, tc.input.IsValid(), tc.wantErr, tc.wantErr, tc.wantValid)
			}
			if gotName.String() != tc.wantName || tc.input.String() != tc.wantName {
				t.Fatalf("header nominal/diagnostic = (%q, %q), want (%q, %q)", gotName.String(), tc.input.String(), tc.wantName, tc.wantName)
			}
			if !tc.wantValid {
				if gotName != (core.HTTPHeaderName{}) {
					t.Fatalf("refused header capability = %+v, want exact zero", gotName)
				}
				return
			}
			if err := gotName.Validate(); err != nil {
				t.Fatalf("admitted header nominal validation = %v, want nil", err)
			}
			const value = "exact admitted value"
			fields := make(http.Header)
			fields.Set(gotName.String(), value)
			if got := fields.Get(tc.wantName); got != value || len(fields) != 1 || http.CanonicalHeaderKey(gotName.String()) != tc.wantName {
				t.Fatalf("Go header handoff = (%q, %v), want one canonical %q field carrying %q", got, fields, tc.wantName, value)
			}
		})
	}
}

func TestStandardMediaTypeExhaustsCompleteByteDomain(t *testing.T) {
	t.Parallel()
	type mediaCase struct {
		name      string
		input     StandardMediaType
		wantName  string
		wantValid bool
		wantErr   error
	}
	cases := [math.MaxUint8 + 1]mediaCase{}
	for ordinal := range cases {
		cases[ordinal] = mediaCase{name: fmt.Sprintf("unpublished media ordinal %d cannot acquire a representation", ordinal), input: StandardMediaType(ordinal), wantErr: core.ErrExchangeContract}
	}
	cases[StandardMediaTypeJSON] = mediaCase{name: "JSON cannot be substituted with valid plain text", input: StandardMediaTypeJSON, wantName: core.HTTPMediaTypeJSON().String(), wantValid: true}
	cases[StandardMediaTypePlainText] = mediaCase{name: "plain text cannot be substituted with valid JSON", input: StandardMediaTypePlainText, wantName: plainTextMediaTypeText, wantValid: true}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotErr := tc.input.Validate()
			gotMedia, gotMediaErr := tc.input.HTTPMediaType()
			if !errors.Is(gotErr, tc.wantErr) || !errors.Is(gotMediaErr, tc.wantErr) || tc.input.IsValid() != tc.wantValid {
				t.Fatalf("media validation/projection/validity = (%v, %v, %t), want (%v, %v, %t)", gotErr, gotMediaErr, tc.input.IsValid(), tc.wantErr, tc.wantErr, tc.wantValid)
			}
			if gotMedia.String() != tc.wantName || tc.input.String() != tc.wantName {
				t.Fatalf("media nominal/diagnostic = (%q, %q), want (%q, %q)", gotMedia.String(), tc.input.String(), tc.wantName, tc.wantName)
			}
			if !tc.wantValid {
				if gotMedia != (core.HTTPMediaType{}) {
					t.Fatalf("refused media capability = %+v, want exact zero", gotMedia)
				}
				return
			}
			if err := gotMedia.Validate(); err != nil {
				t.Fatalf("admitted media nominal validation = %v, want nil", err)
			}
			base, parameters, err := mime.ParseMediaType(gotMedia.String())
			if err != nil || base != tc.wantName || len(parameters) != 0 {
				t.Fatalf("Go media handoff = (%q, %v, %v), want (%q, no parameters, nil)", base, parameters, err, tc.wantName)
			}
		})
	}
}

var (
	_ core.OffWireEnum = StandardHeaderUnknown
	_ core.OffWireEnum = StandardMediaTypeUnknown
)
