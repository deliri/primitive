package exchange

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func BenchmarkParseIdempotencyKeyMaximum(b *testing.B) {
	input := strings.Repeat("~", IdempotencyKeyMaximumBytes)
	b.ReportAllocs()
	for b.Loop() {
		got, err := ParseIdempotencyKey(input)
		if err != nil || got.String() != input || got.IsZero() {
			b.Fatalf("maximum key = (%q, %v), want exact supplied key", got.String(), err)
		}
	}
}

func BenchmarkReceiveBasicAuthorizationCustody(b *testing.B) {
	b.ReportAllocs()
	request := BasicAuthorizationRequest{
		Identity: BasicAuthorizationIdentity(strings.Repeat("i", BasicAuthorizationIdentityMaximumBytes)),
		Secret:   bytes.Repeat([]byte{'s'}, BasicAuthorizationSecretMaximumBytes),
	}
	header, err := NewBasicAuthorizationHeader(request)
	if err != nil {
		b.Fatal(err)
	}
	value, err := header.Values[0].Value()
	if err != nil {
		b.Fatal(err)
	}
	cases := []struct {
		name    string
		value   string
		wantErr error
	}{
		{name: "maximum_credentials", value: value},
		{name: "partial_base64_refusal", value: value[:len(value)-1] + "!", wantErr: core.ErrExchangeRequest},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			wire := httptest.NewRequest(http.MethodGet, "/", nil)
			wire.Header.Set(header.Name.String(), tc.value)
			call, err := NewSocketServerCall(httptest.NewRecorder(), wire)
			if err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			for b.Loop() {
				got, err := ReceiveBasicAuthorization(call)
				if !errors.Is(err, tc.wantErr) {
					b.Fatalf("credential receive error = %v, want %v", err, tc.wantErr)
				}
				if tc.wantErr != nil {
					if got.Identity != "" || got.Secret != nil || !errors.Is(err, core.ErrExchangeContract) {
						b.Fatalf("credential refusal = (%v, %v), want zero and typed contract refusal", got, err)
					}
					continue
				}
				if got.Identity != request.Identity || !bytes.Equal(got.Secret, request.Secret) {
					b.Fatalf("identity/secret preserved=%t/%t, want true/true", got.Identity == request.Identity, bytes.Equal(got.Secret, request.Secret))
				}
				clear(got.Secret)
			}
		})
	}
}
