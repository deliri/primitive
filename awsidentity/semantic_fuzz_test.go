package awsidentity

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func FuzzAWSRequestQueryClosure(f *testing.F) {
	request := awsRequest(f)
	if err := request.Validate(); err != nil {
		f.Fatalf("request seed validation = %v, want nil", err)
	}
	base := request.endpoint.HTTPURL()
	for _, suffix := range []string{"", "&", amazonSecurityTokenQuery + "=" + url.QueryEscape("session+/="), amazonActionQuery + "=" + amazonActionValue, "%zz=value", "Future=%zz", "Future=a;b"} {
		f.Add(suffix)
	}
	f.Fuzz(func(t *testing.T, suffix string) {
		raw := base.String() + "&" + suffix
		input := RequestInput{SignedURL: raw, Audience: request.audience, Policy: request.policy}
		got, gotErr := NewRequest(input)
		endpoint, endpointErr := core.ParseHTTPEndpoint(raw)
		// The required query was fixed and validated before fuzzing. Appending
		// external bytes may add only one optional, unique, nonempty session token;
		// ParseQuery's error must never be discarded. This is a one-fact invariant,
		// not a second implementation of credential or endpoint validation.
		wantValid := endpointErr == nil
		if wantValid {
			parsed := endpoint.HTTPURL()
			query, err := url.ParseQuery(parsed.RawQuery)
			wantValid = err == nil
			original := base.Query()
			for name, values := range original {
				if !slices.Equal(query[name], values) {
					wantValid = false
				}
			}
			for name, values := range query {
				if _, known := original[name]; known {
					continue
				}
				if name != amazonSecurityTokenQuery || len(values) != 1 || values[0] == "" {
					wantValid = false
				}
			}
		}
		if !wantValid {
			if !errors.Is(gotErr, core.ErrAWSIdentityContract) || got != (Request{}) || !errors.Is(input.Validate(), core.ErrAWSIdentityContract) {
				t.Fatalf("query closure rejected = (%v,%v), want zero typed refusal", got, gotErr)
			}
			return
		}
		if gotErr != nil || got.Validate() != nil || input.Validate() != nil || got.endpoint.String() != raw || got.audience != input.Audience || got.policy != input.Policy {
			t.Fatalf("query closure accepted = (%v,%v), want exact retained capability", got, gotErr)
		}
	})
}

func FuzzAWSAcquireTokenProjection(f *testing.F) {
	tokenPattern := regexp.MustCompile(`^[A-Za-z0-9._~+/-]+=*$`)
	for _, value := range []string{awsTestBearer, "a", strings.Repeat("a", TokenMaximumBytes)} {
		document := awsProviderDocument(value)
		seed, err := amazonResponseToken(awsProviderBytes(f, document))
		if err != nil || seed.Validate() != nil {
			f.Fatalf("provider seed = %v, want valid typed token", err)
		}
		disclosed, err := seed.BearerValue()
		if err != nil {
			f.Fatalf("seed disclosure error = %v, want nil", err)
		}
		f.Add(strings.TrimPrefix(disclosed, bearerPrefix))
	}
	for _, value := range []string{"", "a=b", "a&b", "a\r\nb", "\xff", strings.Repeat("a", TokenMaximumBytes+1)} {
		f.Add(value)
	}
	f.Fuzz(func(t *testing.T, value string) {
		// Keep secondary XML fixture work bounded; oversized fuzz text still
		// reaches Acquire as an oversized response and must fail at its read bound.
		var data []byte
		var reader io.Reader
		var length int64
		wantValid := len(value) > 0 && len(value) <= TokenMaximumBytes && tokenPattern.MatchString(value)
		if len(value) > TokenMaximumBytes {
			reader = strings.NewReader(value)
			length = int64(len(value))
		} else {
			data = awsProviderBytes(t, awsProviderDocument(value))
			reader = bytes.NewReader(data)
			length = int64(len(data))
		}
		body := &awsObservedBody{reader: reader}
		transport := &awsResponseTransport{body: body, status: http.StatusOK, length: length}
		got, gotErr := Acquire(t.Context(), awsClient(t, transport), awsRequest(t))
		if !wantValid {
			if !errors.Is(gotErr, core.ErrAWSIdentityContract) || got != (Token{}) {
				t.Fatalf("Acquire token refusal = (%v,%v), want zero typed refusal", got, gotErr)
			}
		} else {
			disclosed, err := got.BearerValue()
			if gotErr != nil || err != nil || got.Validate() != nil || disclosed != bearerPrefix+value {
				t.Fatalf("Acquire token projection = (%q,%v,%v), want exact mutated text", disclosed, gotErr, err)
			}
		}
		if transport.calls != 1 || body.closes != 1 || body.bytes > AmazonResponseMaximumBytes+1 {
			t.Fatalf("Acquire effect calls/closes/bytes = %d/%d/%d, want 1/1/bounded", transport.calls, body.closes, body.bytes)
		}
	})
}

func FuzzAWSAcquireProviderEnvelopeMutations(f *testing.F) {
	cases := awsProviderCases()
	for index, tc := range cases {
		document := awsProviderDocument(awsTestBearer)
		if tc.change != nil {
			tc.change(&document)
		}
		f.Add(uint8(index), uint8(0))
	}
	f.Fuzz(func(t *testing.T, selector, cut uint8) {
		tc := cases[int(selector)%len(cases)]
		document := awsProviderDocument(awsTestBearer)
		original := awsProviderBytes(t, document)
		if tc.change != nil {
			tc.change(&document)
		}
		data := awsProviderBytes(t, document)
		if tc.change != nil && bytes.Equal(original, data) {
			t.Fatalf("envelope mutation=%x, want different from %x", data, original)
		}
		wantErr := tc.wantErr
		if cut != 0 {
			// A strict prefix cannot contain the complete root closing element.
			data = data[:int(cut)*len(data)/256]
			wantErr = core.ErrAWSIdentityContract
		}
		body := &awsObservedBody{reader: bytes.NewReader(data)}
		transport := &awsResponseTransport{body: body, status: http.StatusOK, length: int64(len(data))}
		got, gotErr := Acquire(t.Context(), awsClient(t, transport), awsRequest(t))
		if !errors.Is(gotErr, wantErr) {
			t.Fatalf("Acquire envelope error = %v, want %v for %s", gotErr, wantErr, tc.name)
		}
		if wantErr != nil {
			if got != (Token{}) {
				t.Fatalf("envelope refusal = %v, want zero", got)
			}
		} else {
			value, err := got.BearerValue()
			if err != nil || got.Validate() != nil || value != bearerPrefix+tc.wantToken {
				t.Fatalf("envelope disclosure = (%q,%v), want exact source token", value, err)
			}
		}
		if transport.calls != 1 || body.closes != 1 || body.bytes != len(data) {
			t.Fatalf("envelope effect = %d/%d/%d, want 1/1/%d", transport.calls, body.closes, body.bytes, len(data))
		}
	})
}

// Differential stream inspection uses encoding/xml tokens, independently of
// production's struct projection. For accepted raw documents, it pins the exact
// token text and namespace seen on the wire. Named typed mutations above also
// require acceptance of valid documents and rejection of every invalid slice.
func FuzzAWSProviderResponseSemanticClosure(f *testing.F) {
	canonical := awsProviderBytes(f, awsProviderDocument(awsTestBearer))
	admitted, err := amazonResponseToken(canonical)
	if err != nil || admitted.Validate() != nil {
		f.Fatalf("provider canonical seed error = %v, want nil", err)
	}
	for _, seed := range [][]byte{canonical, append(bytes.Clone(canonical), []byte("<Future/>")...), append(bytes.Clone(canonical), []byte("trailing text")...), nil, []byte("<truncated"), bytes.Repeat([]byte{'x'}, AmazonResponseMaximumBytes+1)} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		body := &awsObservedBody{reader: bytes.NewReader(data)}
		transport := &awsResponseTransport{body: body, status: http.StatusOK, length: int64(len(data))}
		got, gotErr := Acquire(t.Context(), awsClient(t, transport), awsRequest(t))
		if transport.calls != 1 || body.closes != 1 || body.bytes > AmazonResponseMaximumBytes+1 {
			t.Fatalf("raw provider effect = %d/%d/%d, want 1/1/bounded", transport.calls, body.closes, body.bytes)
		}
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrAWSIdentityContract) || got != (Token{}) {
				t.Fatalf("raw provider refusal = (%v,%v), want zero typed refusal", got, gotErr)
			}
			if bytes.Equal(data, canonical) {
				t.Fatalf("canonical provider refusal=%v, want nil", gotErr)
			}
			return
		}
		decoder := xml.NewDecoder(bytes.NewReader(data))
		tokenName := awsProviderName[amazonTokenElement]()
		var value strings.Builder
		var depth, tokenDepth, count, roots int
		for {
			token, err := decoder.Token()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				t.Fatalf("accepted XML token stream error = %v, want well formed", err)
			}
			switch typed := token.(type) {
			case xml.StartElement:
				depth++
				if depth == 1 {
					roots++
					if typed.Name != awsProviderName[amazonResponse]() {
						t.Fatalf("accepted root = %v, want provider root", typed.Name)
					}
				}
				if typed.Name == tokenName {
					count++
					tokenDepth = depth
				}
			case xml.EndElement:
				if depth == tokenDepth {
					tokenDepth = 0
				}
				depth--
			case xml.CharData:
				if depth == 0 && len(bytes.Trim(typed, " \t\r\n")) != 0 {
					t.Fatalf("outside-root text=%q, want only XML padding", typed)
				}
				if tokenDepth != 0 {
					value.Write(typed)
				}
			}
		}
		disclosed, err := got.BearerValue()
		if roots != 1 || count != 1 || depth != 0 || got.Validate() != nil || err != nil || disclosed != bearerPrefix+value.String() {
			t.Fatalf("accepted provider token count/depth/disclosure = %d/%d/%q (error %v), want exact unique wire token", count, depth, disclosed, err)
		}
	})
}
