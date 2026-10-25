package exchange

import (
	"errors"
	"math"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func signedContentLengthFixture(t testing.TB, bytes uint64) SignedContentLength {
	t.Helper()
	extent, err := core.NewByteLength(bytes)
	if err != nil {
		t.Fatal(err)
	}
	length, err := NewSignedContentLength(extent)
	if err != nil {
		t.Fatal(err)
	}
	return length
}

// A signed extent is the only thing that stops a write capability holder from
// sending more bytes than the issuer authorized. Each row proves the length is
// inside the signature: the independent derivation and the official SDK agree,
// and the same request with one more byte no longer verifies.
func TestV4PresignSignedContentLengthLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		wantHeaders string
		bytes       uint64
		signed      bool
		contentType bool
	}{
		{name: "absent length keeps the unsigned-extent header set", wantHeaders: "host"},
		{name: "zero-byte body is a signed extent, not an absent one", bytes: 0, signed: true, wantHeaders: "content-length;host"},
		{name: "one-byte body is signed", bytes: 1, signed: true, wantHeaders: "content-length;host"},
		{name: "ordinary media extent is signed", bytes: 5 << 20, signed: true, wantHeaders: "content-length;host"},
		{name: "signed length sorts before a signed media type", bytes: 4096, signed: true, contentType: true, wantHeaders: "content-length;content-type;host"},
		{name: "largest Go size is signed exactly", bytes: math.MaxInt64, signed: true, wantHeaders: "content-length;host"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			intent := signingFixture(t)
			intent.Method = MethodPut
			if tc.contentType {
				intent.ContentType = core.HTTPMediaTypeOctetStream()
			}
			if tc.signed {
				intent.ContentLength = signedContentLengthFixture(t, tc.bytes)
			}
			got, err := PresignV4(t.Context(), intent)
			if err != nil {
				t.Fatalf("PresignV4() error = %v, want nil", err)
			}
			signedURL := got.HTTPURL()
			if gotHeaders := signedURL.Query().Get(core.SignatureV4SignedHeadersQuery); gotHeaders != tc.wantHeaders {
				t.Fatalf("signed headers = %q, want %q", gotHeaders, tc.wantHeaders)
			}
			if !signingProofMatches(intent, got) {
				t.Fatal("independent canonical derivation = mismatch, want agreement")
			}
			// The SDK treats zero as an unknown length and never signs it; the
			// independent derivation above is the oracle for that one row.
			if tc.bytes > 0 || !tc.signed {
				if want := v4OfficialSDKOracle(t, intent); got.String() != want {
					t.Fatalf("Go signature=%s SDK=%s", got.String(), want)
				}
			}
			if !tc.signed || tc.bytes == math.MaxInt64 {
				return
			}
			larger := intent
			larger.ContentLength = signedContentLengthFixture(t, tc.bytes+1)
			if signingProofMatches(larger, got) {
				t.Fatal("one more byte retained signature validity, want refusal")
			}
		})
	}
}

// Only the zero value means "no signed length". A present flag cleared on a
// carried extent would sign a request the caller believes is bounded.
func TestV4PresignSignedContentLengthRefusesForgedAbsence(t *testing.T) {
	t.Parallel()
	extent, err := core.NewByteLength(7)
	if err != nil {
		t.Fatal(err)
	}
	intent := signingFixture(t)
	intent.Method = MethodPut
	intent.ContentLength = SignedContentLength{bytes: extent}
	got, err := PresignV4(t.Context(), intent)
	if !errors.Is(err, core.ErrExchangeRequest) || got != (core.HTTPEndpoint{}) {
		t.Fatalf("PresignV4(forged absence) = (%v, %v), want zero and %v", got, err, core.ErrExchangeRequest)
	}
}
