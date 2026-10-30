package exchange

import (
	"crypto/sha256"
	"encoding/hex"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"math"
	"net/url"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestMultipartNativeFormattingDoesNotSupplyAdmissionAuthority(t *testing.T) {
	t.Parallel()
	for raw := range 256 {
		kind := MultipartPartKind(raw)
		want := ""
		switch kind {
		case MultipartPartField:
			want = "field"
		case MultipartPartFile:
			want = "file"
		}
		var wantErr error
		if want == "" {
			wantErr = core.ErrExchangeContract
		}
		if !errors.Is(kind.Validate(), wantErr) || kind.IsValid() != (wantErr == nil) {
			t.Fatalf("framing admission byte%d = (%v,%t), want (%v,%t)", raw, kind.Validate(), kind.IsValid(), wantErr, wantErr == nil)
		}
		if got := fmt.Sprintf("%s", kind); got != want {
			t.Fatalf("native framing diagnostic byte%d = %q, want %q after independent typed admission", raw, got, want)
		}
	}
}

func TestOfficialSDKNativeJSONValueGrammarBoundary(t *testing.T) {
	t.Parallel()
	canonical, err := json.Marshal(struct {
		Value string `json:"value"`
	}{Value: "native grammar"})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name           string
		payload        []byte
		representation OfficialSDKResponseRepresentation
		wantErr        error
	}{
		{name: "typed native encoder output is accepted", representation: OfficialSDKResponseRepresentationJSON, payload: canonical},
		{name: "empty response is neutral before JSON grammar", representation: OfficialSDKResponseRepresentationJSON},
		{name: "binary response retains nonJSON bytes", representation: OfficialSDKResponseRepresentationBinary, payload: []byte("{")},
		{name: "incomplete JSON is refused", representation: OfficialSDKResponseRepresentationJSON, payload: []byte("{"), wantErr: core.ErrJSONContract},
		{name: "invalid UTF8 JSON is refused", representation: OfficialSDKResponseRepresentationJSON, payload: []byte{'"', 0xff, '"'}, wantErr: core.ErrJSONContract},
		{name: "second JSON value is refused", representation: OfficialSDKResponseRepresentationJSON, payload: append(append([]byte{}, canonical...), canonical...), wantErr: core.ErrJSONContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotErr := validateOfficialSDKResponsePayload(tc.representation, tc.payload)
			if !errors.Is(gotErr, tc.wantErr) || tc.wantErr != nil && !errors.Is(gotErr, core.ErrExchangeResponse) {
				t.Fatalf("native grammar boundary error = %v, want %v and owning response class on refusal", gotErr, tc.wantErr)
			}
		})
	}
}

func TestSecretHeaderSchemeExhaustsNativeByteDomain(t *testing.T) {
	t.Parallel()
	for raw := range 256 {
		scheme := SecretHeaderScheme(raw)
		wantValid := scheme == SecretHeaderLiteral || scheme == SecretHeaderBearer
		wantLabel := core.UnknownEnumDiagnostic
		if scheme == SecretHeaderLiteral {
			wantLabel = "literal"
		}
		if scheme == SecretHeaderBearer {
			wantLabel = "bearer"
		}
		var wantErr error
		if !wantValid {
			wantErr = core.ErrExchangeContract
		}
		if scheme.IsValid() != wantValid || !errors.Is(scheme.Validate(), wantErr) || scheme.String() != wantLabel {
			t.Fatalf("scheme byte%d = (%t,%v,%q), want (%t,%v,%q)", raw, scheme.IsValid(), scheme.Validate(), scheme.String(), wantValid, wantErr, wantLabel)
		}
	}
}

// The public request preflight already refuses this input. This direct internal
// constructor ratchet proves a bypassed preflight returns a typed refusal rather
// than panicking or publishing a partially built native request. No Do occurs.
func TestAggregateUnknownSecretSchemeRefusesWithoutNativeRequest(t *testing.T) {
	t.Parallel()
	target, err := core.ParseHTTPEndpoint("https://provider.example.test/owned")
	if err != nil {
		t.Fatal(err)
	}
	name, err := core.ParseHTTPHeaderName("X-Owned-Key")
	if err != nil {
		t.Fatal(err)
	}
	got, gotErr := newAggregateHTTPRequest(t.Context(), aggregateRequest{
		target: target, semantics: RequestSemantics{Method: MethodGet, Replay: ReplaySingleAttempt},
		secretHeaders: []SecretHeader{{Name: name, Value: []byte("synthetic"), Scheme: SecretHeaderScheme(255)}},
	})
	if got != nil || !errors.Is(gotErr, core.ErrExchangeRequest) || !errors.Is(gotErr, core.ErrExchangeContract) {
		t.Fatalf("unknown scheme constructor = (request present%t,%v), want absent/ExchangeRequest/ExchangeContract", got != nil, gotErr)
	}
}

func TestV4QueryEncodedIngressPreservesEveryNativeByte(t *testing.T) {
	t.Parallel()
	for raw := range 256 {
		decoded := string([]byte{byte(raw)})
		encoded := url.QueryEscape(decoded)
		name, nameErr := parseV4QueryName(encoded)
		value, valueErr := parseV4QueryValue(encoded)
		if nameErr != nil || valueErr != nil || name.String() != decoded || value.String() != decoded {
			t.Fatalf("encoded byte%d closure = (%v,%v,name equal%t,value equal%t), want nil/nil/true/true", raw, nameErr, valueErr, name.String() == decoded, value.String() == decoded)
		}
	}
	for _, decoded := range []string{"", "a+b = % /", "雪", "a\x00\xffb", strings.Repeat("x", 32769)} {
		encoded := v4QueryEncoding(decoded)
		name, nameErr := parseV4QueryName(encoded)
		value, valueErr := parseV4QueryValue(encoded)
		if nameErr != nil || valueErr != nil || name.String() != decoded || value.String() != decoded || v4QueryEncoding(name.String()) != encoded || v4QueryEncoding(value.String()) != encoded {
			t.Fatalf("native query closure at%dbytes = (%v,%v), want exact decoded/canonical bytes and nil", len(decoded), nameErr, valueErr)
		}
	}
	for _, encoded := range []string{"%", "%0", "%GG", "prefix%0tail"} {
		name, nameErr := parseV4QueryName(encoded)
		value, valueErr := parseV4QueryValue(encoded)
		_, nativeErr := url.QueryUnescape(encoded)
		var nameNative, valueNative url.EscapeError
		if nativeErr == nil || name != "" || value != "" || !errors.Is(nameErr, core.ErrExchangeRequest) || !errors.Is(valueErr, core.ErrExchangeRequest) || !errors.As(nameErr, &nameNative) || !errors.As(valueErr, &valueNative) {
			t.Fatalf("malformed native query = (%v,%v), want zero values and ExchangeRequest retaining native EscapeError", nameErr, valueErr)
		}
	}
}

func TestV4TimestampOwnsCalendarAndProducerNanosecondEdges(t *testing.T) {
	t.Parallel()
	for _, nanos := range []int64{math.MinInt64, -1, 0, 1, math.MaxInt64} {
		text, err := temporal.InstantFromNanoseconds(nanos).CompactUTCSeconds()
		if err != nil {
			t.Fatal(err)
		}
		got, gotErr := parseV4Timestamp(text)
		if gotErr != nil || got.Validate() != nil || got.String() != text {
			t.Fatalf("timestamp producer closure at%d = (%v,equal%t), want nil/true", nanos, gotErr, got.String() == text)
		}
	}
	for _, text := range []string{"", "19700101T000000", "19700101T000000z", "19700101T000060Z", "19700230T000000Z", "19700101T000000.0Z", "16770921T001242Z", "22620411T234717Z"} {
		got, gotErr := parseV4Timestamp(text)
		if got != "" || !errors.Is(gotErr, core.ErrExchangeRequest) || (v4Timestamp(text)).Validate() == nil {
			t.Fatalf("timestamp refusal = (zero%t,%v), want true/ExchangeRequest and invalid unadmitted value", got == "", gotErr)
		}
	}
}

func TestV4CredentialScopeOwnsNativeCalendarAndExactSeparators(t *testing.T) {
	t.Parallel()
	for _, text := range []string{"19700101/r/s/aws4_request", "20000229/region/service/aws4_request", "16770921/r/s/aws4_request", "22620411/r/s/aws4_request", "19700101/\xff/\x00/aws4_request", "19700101/" + strings.Repeat("r", 32769) + "/s/aws4_request"} {
		got, gotErr := parseV4CredentialScope(text)
		if gotErr != nil || got.Validate() != nil || got.String() != text {
			t.Fatalf("scope closure at%dbytes = (%v,equal%t), want nil/true", len(text), gotErr, got.String() == text)
		}
	}
	for _, text := range []string{"", "1970010/r/s/aws4_request", "19700230/r/s/aws4_request", "19700101//s/aws4_request", "19700101/r//aws4_request", "19700101/r/s/other", "19700101/r/s/aws4_request/extra", "19700101/r\r/s/aws4_request", "19700101/r/s\n/aws4_request"} {
		got, gotErr := parseV4CredentialScope(text)
		if got != "" || !errors.Is(gotErr, core.ErrExchangeRequest) || (v4CredentialScope(text)).Validate() == nil {
			t.Fatalf("scope refusal = (zero%t,%v), want true/ExchangeRequest and invalid unadmitted value", got == "", gotErr)
		}
	}
}

func TestV4CredentialSeparatesBorrowedKeyFromScopeAtRightBoundary(t *testing.T) {
	t.Parallel()
	for _, key := range []string{"key", "key/with/slashes", "/", "\x00\xff\r\n", strings.Repeat("k", 32769)} {
		text := key + "/19700101/r/s/aws4_request"
		got, gotErr := parseV4Credential(text)
		if gotErr != nil || got.Validate() != nil || got.String() != text {
			t.Fatalf("credential closure at%dkeybytes = (%v,equal%t), want nil/true", len(key), gotErr, got.String() == text)
		}
	}
	for _, text := range []string{"", "key", "/19700101/r/s/aws4_request", "key/19700101/r/s/other", "key/19700230/r/s/aws4_request", "key/19700101//s/aws4_request"} {
		got, gotErr := parseV4Credential(text)
		if got != "" || !errors.Is(gotErr, core.ErrExchangeRequest) || (v4Credential(text)).Validate() == nil {
			t.Fatalf("credential refusal = (zero%t,%v), want true/ExchangeRequest and invalid unadmitted value", got == "", gotErr)
		}
	}
}

func TestV4SignedHeaderNamesOwnSortedUniqueLowercaseHostSet(t *testing.T) {
	t.Parallel()
	for _, text := range []string{"host", "content-length;host", "content-type;host;x-owned", "a;host;z"} {
		got, gotErr := parseV4SignedHeaders(text)
		if gotErr != nil || got.Validate() != nil || got.String() != text {
			t.Fatalf("signed header closure = (%v,equal%t), want nil/true", gotErr, got.String() == text)
		}
	}
	for _, text := range []string{"", "x-owned", "Host", "host;host", "z;host", "host;", ";host", "host;x bad", "host;x\n", "authorization;host", "host;user-agent"} {
		got, gotErr := parseV4SignedHeaders(text)
		if got != "" || !errors.Is(gotErr, core.ErrExchangeRequest) || (v4SignedHeaders(text)).Validate() == nil {
			t.Fatalf("signed header refusal = (zero%t,%v), want true/ExchangeRequest and invalid unadmitted value", got == "", gotErr)
		}
	}
}

// The pinned SDK sorts decoded query keys. Native byte keys can therefore
// disagree with its order: the owning canonical contract sorts encoded names.
// Pin that entire query independently, then use ordinary ASCII keys to retain
// full SDK crypto agreement and a one-byte input-to-MAC mutation at both edges.
func TestV4NativeByteNameOrderAndSDKValueOracleAtInstantEdges(t *testing.T) {
	t.Parallel()
	for _, nanos := range []int64{math.MinInt64, math.MaxInt64} {
		intent := signingFixture(t)
		intent.SignedAt = temporal.InstantFromNanoseconds(nanos)
		intent.AccessKey = []byte("key/with/slashes")
		name, err := core.ParseHTTPHeaderName("X-Owned-Proof")
		if err != nil {
			t.Fatal(err)
		}
		first, err := NewHeaderValue("  first   middle  ")
		if err != nil {
			t.Fatal(err)
		}
		second, err := NewHeaderValue("second")
		if err != nil {
			t.Fatal(err)
		}
		intent.Headers = Headers{Values: []Header{{Name: name, Values: []HeaderValue{first, second}}}}
		if nanos == math.MaxInt64 {
			intent.Region, intent.Service = "\xff", "\x00"
		}
		target := intent.Target.HTTPURL()
		target.RawQuery = "%00=%FF&%FF=%00&empty=&=value"
		intent.Target, err = core.ParseHTTPEndpoint(target.String())
		if err != nil {
			t.Fatal(err)
		}
		got, gotErr := PresignV4(t.Context(), intent)
		if gotErr != nil {
			t.Fatalf("native byte-name signing at%d error = %v, want nil", nanos, gotErr)
		}
		stamp, err := intent.SignedAt.CompactUTCSeconds()
		if err != nil {
			t.Fatal(err)
		}
		scope := stamp[:8] + "/" + intent.Region + "/" + intent.Service + "/aws4_request"
		wantQuery := "=value&%00=%FF&%FF=%00&X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Credential=" + url.QueryEscape(string(intent.AccessKey)+"/"+scope) + "&X-Amz-Date=" + stamp + "&X-Amz-SignedHeaders=host%3Bx-owned-proof&empty="
		query, signature, present := strings.Cut(got.HTTPURL().RawQuery, "&X-Amz-Signature=")
		mac, macErr := hex.DecodeString(signature)
		if !present || query != wantQuery || macErr != nil || len(signature) != hex.EncodedLen(sha256.Size) || len(mac) != sha256.Size {
			t.Fatalf("native byte-name query/mac at%d = (query equal%t,signature present%t,%v,%dbytes), want true/true/nil/%d", nanos, query == wantQuery, present, macErr, len(mac), sha256.Size)
		}
		previous := signature
		for _, rawQuery := range []string{"zero=%00&high=%FF&empty=", "zero=%01&high=%FF&empty="} {
			target.RawQuery = rawQuery
			intent.Target, err = core.ParseHTTPEndpoint(target.String())
			if err != nil {
				t.Fatal(err)
			}
			got, gotErr = PresignV4(t.Context(), intent)
			if gotErr != nil || got.String() != v4OfficialSDKOracle(t, intent) {
				t.Fatalf("native byte-value URL equals SDK at%d = false, want true; error%v", nanos, gotErr)
			}
			_, signature, present = strings.Cut(got.HTTPURL().RawQuery, "&X-Amz-Signature=")
			mac, macErr = hex.DecodeString(signature)
			if !present || signature == previous || macErr != nil || len(signature) != hex.EncodedLen(sha256.Size) || len(mac) != sha256.Size {
				t.Fatalf("changed native input mac at%d = (present%t,changed%t,%v,%dbytes), want true/true/nil/%d", nanos, present, signature != previous, macErr, len(mac), sha256.Size)
			}
			previous = signature
		}
	}
}

func FuzzV4ScalarNativeAndPublicSigningClosure(f *testing.F) {
	seed := signingFixture(f)
	canonical, err := seed.SignedAt.CompactUTCSeconds()
	if err != nil {
		f.Fatal(err)
	}
	parsed, err := parseV4QueryValue(url.QueryEscape(canonical))
	if err != nil {
		f.Fatal(err)
	}
	f.Add(v4QueryEncoding(parsed.String()), seed.Region, seed.Service, string(seed.AccessKey), int64(0))
	f.Add("%00%FF", "r", "s", "key/with/slashes", int64(math.MinInt64))
	f.Add("%GG", "r", "s", "key", int64(math.MaxInt64))
	f.Add("", "bad/region", "s", "", int64(0))
	f.Fuzz(func(t *testing.T, encoded, region, service, key string, nanos int64) {
		name, nameErr := parseV4QueryName(encoded)
		value, valueErr := parseV4QueryValue(encoded)
		native, nativeErr := url.QueryUnescape(encoded)
		if nativeErr != nil {
			var nativeName, nativeValue url.EscapeError
			if name != "" || value != "" || !errors.Is(nameErr, core.ErrExchangeRequest) || !errors.Is(valueErr, core.ErrExchangeRequest) || !errors.As(nameErr, &nativeName) || !errors.As(valueErr, &nativeValue) {
				t.Fatalf("native escape refusal = (%v,%v), want typed causes/zero values", nameErr, valueErr)
			}
		} else {
			if nameErr != nil || valueErr != nil || name.String() != native || value.String() != native {
				t.Fatalf("native decoded equality = (%v,%v,%t,%t), want nil/nil/true/true", nameErr, valueErr, name.String() == native, value.String() == native)
			}
			canonical := v4QueryEncoding(native)
			nameAgain, nameAgainErr := parseV4QueryName(canonical)
			valueAgain, valueAgainErr := parseV4QueryValue(canonical)
			if nameAgainErr != nil || valueAgainErr != nil || nameAgain != name || valueAgain != value || v4QueryEncoding(nameAgain.String()) != canonical || v4QueryEncoding(valueAgain.String()) != canonical {
				t.Fatalf("native canonical closure = (%v,%v), want exact byte value and stable canonical encoding", nameAgainErr, valueAgainErr)
			}
		}
		intent := signingFixture(t)
		intent.Region, intent.Service, intent.AccessKey = region, service, []byte(key)
		intent.SignedAt = temporal.InstantFromNanoseconds(nanos)
		target := intent.Target.HTTPURL()
		target.RawQuery = "input=" + url.QueryEscape(encoded)
		var targetErr error
		intent.Target, targetErr = core.ParseHTTPEndpoint(target.String())
		if targetErr != nil {
			got, gotErr := PresignV4(t.Context(), intent)
			if intent.Target != (core.HTTPEndpoint{}) || !errors.Is(targetErr, core.ErrPrimitiveContract) || got != (core.HTTPEndpoint{}) || !errors.Is(gotErr, core.ErrExchangeRequest) {
				t.Fatalf("owning URL refusal = (%v,%v), want typed native/Exchange refusal and zero endpoints", targetErr, gotErr)
			}
			return
		}
		got, gotErr := PresignV4(t.Context(), intent)
		if intent.Validate() != nil {
			if got != (core.HTTPEndpoint{}) || !errors.Is(gotErr, core.ErrExchangeRequest) {
				t.Fatalf("invalid signing = (zero%t,%v), want true/ExchangeRequest", got == (core.HTTPEndpoint{}), gotErr)
			}
			return
		}
		want := v4OfficialSDKOracle(t, intent)
		_, wantEndpointErr := core.ParseHTTPEndpoint(want)
		if wantEndpointErr != nil {
			if got != (core.HTTPEndpoint{}) || !errors.Is(wantEndpointErr, core.ErrPrimitiveContract) || !errors.Is(gotErr, core.ErrPrimitiveContract) {
				t.Fatalf("signed endpoint owning admission = (%v,%v), want matching typed refusal and zero output", gotErr, wantEndpointErr)
			}
			return
		}
		if gotErr != nil || got.String() != want {
			t.Fatal("admitted native signature equals official SDK = false, want true")
		}
	})
}
