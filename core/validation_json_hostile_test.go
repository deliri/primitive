package core

import (
	"bytes"
	"encoding/json"
	"errors"
	"math"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"
)

type boundaryDisposition uint8

const (
	boundaryReject boundaryDisposition = iota
	boundaryAccept
	fuzzJSONDocumentMaximumBytes         = 4096
	validatedJSONEncodeRatchetComponent  = "encode&<>\x01"
	validatedJSONEncodeAllocationMaximum = 32
	validatedJSONEncodeAllocationSamples = 100
	validatedJSONEncodeBytesMaximum      = 6000
	validatedJSONEncodeMemorySamples     = 100
	strictJSONHostileSharedPrefixBytes   = 4000
	strictJSONHostileComposedPrefixBytes = 3
	strictJSONHostileMaximumSlackBytes   = 1 << 16
	strictJSONDecodeAllocationMaximum    = 1_500_000
	strictJSONDecodeBytesMaximum         = 40 << 20
)

// TestDecodeStrictJSONStructureHostileBoundaryTable is a direct unit ratchet
// for Core's real generic document boundary. Raw bytes are intentional here:
// malformed external JSON has no valid typed representation before parsing.
func TestDecodeStrictJSONStructureHostileBoundaryTable(t *testing.T) {
	t.Parallel()

	cases := []struct {
		wire        func() []byte
		limits      func() StrictJSONLimits
		name        string
		disposition boundaryDisposition
	}{
		{name: "null scalar is structurally valid", wire: func() []byte { return []byte("null") }, disposition: boundaryAccept},
		{name: "false scalar is structurally valid", wire: func() []byte { return []byte("false") }, disposition: boundaryAccept},
		{name: "true scalar is structurally valid", wire: func() []byte { return []byte("true") }, disposition: boundaryAccept},
		{name: "zero scalar is structurally valid", wire: func() []byte { return []byte("0") }, disposition: boundaryAccept},
		{name: "negative scalar is structurally valid", wire: func() []byte { return []byte("-1") }, disposition: boundaryAccept},
		{name: "string scalar is structurally valid", wire: func() []byte { return []byte(`"value"`) }, disposition: boundaryAccept},
		{name: "paired surrogate string is structurally valid", wire: func() []byte { return []byte(`"\ud83d\ude42"`) }, disposition: boundaryAccept},
		{name: "paired surrogate object key is structurally valid", wire: func() []byte { return []byte(`{"\ud83d\ude42":1}`) }, disposition: boundaryAccept},
		{name: "empty object is structurally valid", wire: func() []byte { return []byte("{}") }, disposition: boundaryAccept},
		{name: "empty array is structurally valid", wire: func() []byte { return []byte("[]") }, disposition: boundaryAccept},
		{name: "nested object and array are structurally valid", wire: func() []byte { return []byte(`{"a":[{"b":1}]}`) }, disposition: boundaryAccept},
		{name: "reordered distinct fields are structurally valid", wire: func() []byte { return []byte(`{"z":1,"a":2}`) }, disposition: boundaryAccept},
		{name: "exact maximum object field cardinality is accepted", wire: func() []byte { return strictJSONObject(JSONObjectFieldCountMaximum) }, disposition: boundaryAccept},
		{name: "one below maximum object field cardinality is accepted", wire: func() []byte { return strictJSONObject(JSONObjectFieldCountMaximum - 1) }, disposition: boundaryAccept},
		{
			name:        "one below global maximum distinct object fields are accepted",
			wire:        func() []byte { return strictJSONObject(JSONObjectFieldCountMaximum - 1) },
			limits:      strictJSONGlobalObjectFieldLimits,
			disposition: boundaryAccept,
		},
		{
			name:        "exact global maximum distinct object fields are accepted",
			wire:        func() []byte { return strictJSONObject(JSONObjectFieldCountMaximum) },
			limits:      strictJSONGlobalObjectFieldLimits,
			disposition: boundaryAccept,
		},
		{name: "exact maximum array cardinality is accepted", wire: func() []byte { return strictJSONArray(JSONArrayItemCountMaximum) }, disposition: boundaryAccept},
		{name: "one below maximum array cardinality is accepted", wire: func() []byte { return strictJSONArray(JSONArrayItemCountMaximum - 1) }, disposition: boundaryAccept},
		{name: "one below maximum nesting depth is accepted", wire: func() []byte { return strictJSONNestedArrays(JSONNestingDepthMaximum - 1) }, disposition: boundaryAccept},
		{name: "exact maximum nesting depth is accepted", wire: func() []byte { return strictJSONNestedArrays(JSONNestingDepthMaximum) }, disposition: boundaryAccept},
		{name: "empty document is rejected", wire: func() []byte { return nil }},
		{name: "invalid UTF8 prefix is rejected", wire: func() []byte { return []byte{0xff, '{', '}'} }},
		{name: "one byte above document maximum is rejected", wire: func() []byte { return bytes.Repeat([]byte{' '}, JSONDocumentMaximumBytes+1) }},
		{name: "truncated object is rejected", wire: func() []byte { return []byte(`{"a":1`) }},
		{name: "truncated array is rejected", wire: func() []byte { return []byte(`[1`) }},
		{name: "truncated string is rejected", wire: func() []byte { return []byte(`"value`) }},
		{name: "type-wrong bare identifier is rejected", wire: func() []byte { return []byte("value") }},
		{name: "two top-level scalars are rejected", wire: func() []byte { return []byte("1 2") }},
		{name: "object followed by object is rejected", wire: func() []byte { return []byte("{}{}") }},
		{name: "array followed by null is rejected", wire: func() []byte { return []byte("[]null") }},
		{name: "exact duplicate object field is rejected", wire: func() []byte { return []byte(`{"a":1,"a":2}`) }},
		{name: "ASCII case-folded duplicate field is rejected", wire: func() []byte { return []byte(`{"Name":1,"name":2}`) }},
		{name: "Unicode case-folded duplicate field is rejected", wire: func() []byte { return []byte(`{"K":1,"\u212a":2}`) }},
		{name: "Greek sigma case-folded duplicate field is rejected", wire: func() []byte { return []byte(`{"\u03a3":1,"\u03c2":2}`) }},
		{name: "long S case-folded duplicate field is rejected", wire: func() []byte { return []byte(`{"S":1,"\u017f":2}`) }},
		{name: "nested duplicate field is rejected", wire: func() []byte { return []byte(`{"outer":{"a":1,"a":2}}`) }},
		{name: "duplicate after array value is rejected", wire: func() []byte { return []byte(`{"a":[],"a":{}}`) }},
		{name: "one above maximum object field cardinality is rejected", wire: func() []byte { return strictJSONObject(JSONObjectFieldCountMaximum + 1) }},
		{
			name:   "duplicate in final field at global object maximum is rejected",
			wire:   func() []byte { return strictJSONObjectWithFinalDuplicate(JSONObjectFieldCountMaximum) },
			limits: strictJSONGlobalObjectFieldLimits,
		},
		{
			name:   "one above global maximum object field cardinality is rejected",
			wire:   func() []byte { return strictJSONObject(JSONObjectFieldCountMaximum + 1) },
			limits: strictJSONGlobalObjectFieldLimits,
		},
		{name: "one above maximum array cardinality is rejected", wire: func() []byte { return strictJSONArray(JSONArrayItemCountMaximum + 1) }},
		{name: "one above maximum nesting depth is rejected", wire: func() []byte { return strictJSONNestedArrays(JSONNestingDepthMaximum + 1) }},
		{name: "far above maximum nesting depth is rejected", wire: func() []byte { return strictJSONNestedArrays(JSONNestingDepthMaximum * 2) }},
		{name: "object missing field value is rejected", wire: func() []byte { return []byte(`{"a":}`) }},
		{name: "object missing field name is rejected", wire: func() []byte { return []byte(`{:1}`) }},
		{name: "object trailing comma is rejected", wire: func() []byte { return []byte(`{"a":1,}`) }},
		{name: "array trailing comma is rejected", wire: func() []byte { return []byte(`[1,]`) }},
		{name: "mismatched object close is rejected", wire: func() []byte { return []byte(`{"a":1]`) }},
		{name: "mismatched array close is rejected", wire: func() []byte { return []byte(`[1}`) }},
		{name: "unescaped control byte is rejected", wire: func() []byte { return []byte{'"', 0x01, '"'} }},
		{name: "lone high surrogate string is rejected", wire: func() []byte { return []byte(`"\ud800"`) }},
		{name: "lone low surrogate string is rejected", wire: func() []byte { return []byte(`"\udfff"`) }},
		{name: "lone surrogate object key is rejected", wire: func() []byte { return []byte(`{"\ud800":1}`) }},
		{name: "lone surrogate object value is rejected", wire: func() []byte { return []byte(`{"a":"\udfff"}`) }},
		{name: "high surrogate followed by high surrogate is rejected", wire: func() []byte { return []byte(`"\ud800\ud800"`) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			wire := tc.wire()
			limits := DefaultStrictJSONLimits()
			if tc.limits != nil {
				limits = tc.limits()
			}
			got, gotErr := decodeStrictJSONStructure[json.RawMessage](wire, limits)
			if tc.disposition == boundaryAccept {
				if gotErr != nil {
					t.Fatalf("DecodeStrictJSONStructure() error = %v, want nil", gotErr)
				}
				if !json.Valid(got) {
					t.Fatalf("DecodeStrictJSONStructure() returned invalid JSON %q", got)
				}
				return
			}
			if !errors.Is(gotErr, ErrJSONContract) {
				t.Fatalf("DecodeStrictJSONStructure() error = %v, want %v", gotErr, ErrJSONContract)
			}
			if got != nil {
				t.Fatalf("DecodeStrictJSONStructure() rejected value = %q, want nil", got)
			}
		})
	}
}

func FuzzDecodeStrictJSONAbsolutePathPublicBoundary(f *testing.F) {
	for _, seed := range [][]byte{
		[]byte(`"/tmp/a&b"`),
		[]byte(`"/tmp/a<b"`),
		[]byte(`"/tmp/a>b"`),
		[]byte(`"/tmp/a\u0001b"`),
		[]byte(`"/tmp/a\u000bb"`),
		[]byte(`"/tmp/a\u007fb"`),
		[]byte(`"/tmp/a\u0026b"`),
		[]byte(`"/tmp/a\\u0026b"`),
		[]byte(`"/tmp/a\\\\b"`),
		[]byte(`"/tmp/a\ud800b"`),
		[]byte(`"/tmp/a\udfffb"`),
		[]byte(`"/tmp/a\ud83d\ude42b"`),
		[]byte(`"/tmp/a\ufffdb"`),
		[]byte("null"),
		[]byte(`{"path":"/tmp/a"}`),
		[]byte(`{"path":"/tmp/a","Path":"/tmp/b"}`),
		{0xff},
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, wire []byte) {
		limits := DefaultStrictJSONLimits()
		documentMaximum, gotLimitErr := NewByteCount(fuzzJSONDocumentMaximumBytes)
		if gotLimitErr != nil {
			t.Fatalf(
				"NewByteCount(fuzz document maximum %d) error = %v, want nil",
				fuzzJSONDocumentMaximumBytes,
				gotLimitErr,
			)
		}
		limits.DocumentMaximumBytes = documentMaximum
		got, gotErr := DecodeStrictJSON[AbsolutePath](wire, limits)
		if gotErr != nil {
			if !errors.Is(gotErr, ErrJSONContract) {
				t.Fatalf("DecodeStrictJSON[AbsolutePath]() error = %v, want %v", gotErr, ErrJSONContract)
			}
			if got != (AbsolutePath{}) {
				t.Fatalf("rejected absolute-path fuzz value = %v, want zero", got)
			}
		} else {
			if gotValidateErr := got.Validate(); gotValidateErr != nil {
				t.Fatalf("accepted absolute path Validate() error = %v, want nil", gotValidateErr)
			}
			directWire, gotMarshalErr := got.MarshalJSON()
			if gotMarshalErr != nil || !json.Valid(directWire) {
				t.Fatalf(
					"accepted AbsolutePath.MarshalJSON() = (%q, %v), want valid JSON/nil",
					directWire,
					gotMarshalErr,
				)
			}
			encoded, gotEncodeErr := EncodeValidatedJSON(got, limits)
			if gotEncodeErr != nil || !bytes.Equal(encoded, directWire) {
				t.Fatalf(
					"EncodeValidatedJSON(accepted absolute path) = (%q, %v), want (%q, nil)",
					encoded,
					gotEncodeErr,
					directWire,
				)
			}
			roundTrip, roundTripErr := DecodeStrictJSON[AbsolutePath](encoded, limits)
			if roundTripErr != nil {
				t.Fatalf("DecodeStrictJSON[AbsolutePath](round trip) error = %v, want nil", roundTripErr)
			}
			if roundTrip != got {
				t.Fatalf("absolute-path strict JSON round trip = %v, want %v", roundTrip, got)
			}
		}

	})
}

type strictJSONTextRecord struct {
	Text string `json:"text"`
}

type strictJSONTextRecordWire struct {
	Text string `json:"text"`
}

type strictJSONBenchmarkDocument struct {
	decoded bool
}

type unstableJSONRepresentation struct {
	decoded bool
}

type unstableJSONRepresentationWire struct {
	Value string `json:"value"`
}

func (r strictJSONTextRecord) Validate() error {
	if r.Text == "" {
		return ErrPrimitiveContract
	}
	return nil
}

func (r strictJSONTextRecord) MarshalJSON() ([]byte, error) {
	return json.Marshal(strictJSONTextRecordWire(r))
}

func (d strictJSONBenchmarkDocument) Validate() error {
	if !d.decoded {
		return ErrPrimitiveContract
	}
	return nil
}

func (d *strictJSONBenchmarkDocument) UnmarshalJSON(data []byte) error {
	d.decoded = len(data) != 0
	return nil
}

func (unstableJSONRepresentation) Validate() error {
	return nil
}

func (r unstableJSONRepresentation) MarshalJSON() ([]byte, error) {
	value := "original"
	if r.decoded {
		value = "decoded"
	}
	return json.Marshal(unstableJSONRepresentationWire{Value: value})
}

func (r *unstableJSONRepresentation) UnmarshalJSON(data []byte) error {
	var wire unstableJSONRepresentationWire
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	r.decoded = true
	return nil
}

type encodeValidationOrderRecord struct{}

func (encodeValidationOrderRecord) Validate() error {
	return ErrAttestContract
}

func (encodeValidationOrderRecord) MarshalJSON() ([]byte, error) {
	return nil, ErrCurrencyContract
}

func TestEncodeValidatedJSONValidatesBeforeMarshal(t *testing.T) {
	t.Parallel()

	gotWire, gotErr := EncodeValidatedJSON(
		encodeValidationOrderRecord{},
		DefaultStrictJSONLimits(),
	)
	if !errors.Is(gotErr, ErrAttestContract) {
		t.Fatalf("EncodeValidatedJSON() error = %v, want %v", gotErr, ErrAttestContract)
	}
	if errors.Is(gotErr, ErrCurrencyContract) {
		t.Fatalf("EncodeValidatedJSON() error = %v, do not want marshaler error %v", gotErr, ErrCurrencyContract)
	}
	if gotWire != nil {
		t.Fatalf("EncodeValidatedJSON() wire = %q, want nil", gotWire)
	}
}

func TestMarshalJSONStringRejectsInvalidUTF8(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		value string
	}{
		{name: "lone continuation byte", value: string([]byte{0x80})},
		{name: "maximum invalid byte", value: string([]byte{0xff})},
		{name: "invalid byte before ASCII", value: string([]byte{0xff, 'a'})},
		{name: "invalid byte between ASCII", value: string([]byte{'a', 0xff, 'b'})},
		{name: "invalid byte after ASCII", value: string([]byte{'a', 0xff})},
		{name: "overlong slash sequence", value: string([]byte{0xc0, 0xaf})},
		{name: "truncated two-byte sequence", value: string([]byte{0xc2})},
		{name: "truncated three-byte sequence", value: string([]byte{0xe2, 0x82})},
		{name: "UTF8-encoded surrogate", value: string([]byte{0xed, 0xa0, 0x80})},
		{name: "code point above Unicode maximum", value: string([]byte{0xf4, 0x90, 0x80, 0x80})},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := marshalJSONString(tc.value)
			if !errors.Is(gotErr, ErrJSONContract) {
				t.Fatalf("marshalJSONString() error = %v, want %v", gotErr, ErrJSONContract)
			}
			if got != nil {
				t.Fatalf("marshalJSONString() = %q, want nil", got)
			}

			typedWire, typedErr := (PathComponent{value: tc.value}).MarshalJSON()
			if !errors.Is(typedErr, ErrJSONContract) {
				t.Fatalf("PathComponent.MarshalJSON() error = %v, want %v", typedErr, ErrJSONContract)
			}
			if typedWire != nil {
				t.Fatalf("PathComponent.MarshalJSON() = %q, want nil", typedWire)
			}
		})
	}
}

func TestDecodeJSONStringTokenHostileBoundaryTable(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		want    string
		wire    []byte
		wantErr bool
	}{
		{name: "empty string token is admitted", wire: []byte(`""`)},
		{name: "plain ASCII token is admitted", wire: []byte(`"plain"`), want: "plain"},
		{name: "leading and trailing document whitespace is admitted", wire: []byte(" \n\t\"plain\"\r "), want: "plain"},
		{name: "escaped quote is decoded exactly", wire: []byte(`"a\"b"`), want: `a"b`},
		{name: "escaped backslash is decoded exactly", wire: []byte(`"a\\b"`), want: `a\b`},
		{name: "paired surrogate is decoded exactly", wire: []byte(`"\ud83d\ude42"`), want: "🙂"},
		{name: "literal surrogate text stays literal", wire: []byte(`"\\ud800"`), want: `\ud800`},
		{name: "empty document is rejected", wantErr: true},
		{name: "whitespace-only document is rejected", wire: []byte(" \n\t"), wantErr: true},
		{name: "null is rejected", wire: []byte("null"), wantErr: true},
		{name: "number token is rejected", wire: []byte("1"), wantErr: true},
		{name: "object token is rejected", wire: []byte("{}"), wantErr: true},
		{name: "trailing second document is rejected", wire: []byte(`"plain" true`), wantErr: true},
		{name: "truncated string is rejected", wire: []byte(`"plain`), wantErr: true},
		{name: "invalid UTF8 is rejected", wire: []byte{'"', 0xff, '"'}, wantErr: true},
		{name: "minimum lone high surrogate is rejected", wire: []byte(`"\ud800"`), wantErr: true},
		{name: "maximum lone high surrogate is rejected", wire: []byte(`"\udbff"`), wantErr: true},
		{name: "minimum lone low surrogate is rejected", wire: []byte(`"\udc00"`), wantErr: true},
		{name: "maximum lone low surrogate is rejected", wire: []byte(`"\udfff"`), wantErr: true},
		{name: "high surrogate followed by plain code unit is rejected", wire: []byte(`"\ud800\u0041"`), wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := DecodeJSONStringToken(tc.wire)
			if tc.wantErr {
				if !errors.Is(gotErr, ErrJSONContract) || got != "" {
					t.Fatalf("DecodeJSONStringToken(%q) = (%q, %v), want (empty, %v)", tc.wire, got, gotErr, ErrJSONContract)
				}
				return
			}
			if gotErr != nil || got != tc.want {
				t.Fatalf("DecodeJSONStringToken(%q) = (%q, %v), want (%q, nil)", tc.wire, got, gotErr, tc.want)
			}
		})
	}
}

// TestMarshalJSONStringEscapingHostileBoundaryTable directly ratchets the
// shared string primitive used by every Core JSON string producer. Exact wire
// assertions are intentional: HTML characters must remain literal while
// backslashes, controls, and literal Unicode-escape text remain unambiguous.
func TestMarshalJSONStringEscapingHostileBoundaryTable(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		value string
		want  string
	}{
		{name: "ampersand remains literal", value: "&", want: `"&"`},
		{name: "less-than remains literal", value: "<", want: `"<"`},
		{name: "greater-than remains literal", value: ">", want: `">"`},
		{name: "HTML-sensitive trio remains literal", value: "&<>", want: `"&<>"`},
		{name: "quote remains escaped", value: `"`, want: `"\""`},
		{name: "backslash remains escaped", value: `\`, want: `"\\"`},
		{name: "literal ampersand escape text remains literal", value: `\u0026`, want: `"\\u0026"`},
		{name: "literal comma escape text remains literal", value: `\u002c`, want: `"\\u002c"`},
		{name: "literal period escape text remains literal", value: `\u002e`, want: `"\\u002e"`},
		{name: "two backslashes remain distinct", value: `\\u0026`, want: `"\\\\u0026"`},
		{name: "control byte remains escaped", value: "\x01", want: `"\u0001"`},
		{name: "line separator remains escaped", value: "\u2028", want: `"\u2028"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := marshalJSONString(tc.value)
			if gotErr != nil {
				t.Fatalf("marshalJSONString(%q) error = %v, want nil", tc.value, gotErr)
			}
			if !bytes.Equal(got, []byte(tc.want)) {
				t.Fatalf("marshalJSONString(%q) = %q, want %q", tc.value, got, tc.want)
			}
			if !json.Valid(got) {
				t.Fatalf("marshalJSONString(%q) = %q, want valid JSON", tc.value, got)
			}
			roundTrip, gotUnquoteErr := strconv.Unquote(string(got))
			if gotUnquoteErr != nil || roundTrip != tc.value {
				t.Fatalf(
					"independent string round trip = (%q, %v), want (%q, nil)",
					roundTrip,
					gotUnquoteErr,
					tc.value,
				)
			}
		})
	}
}

func FuzzMarshalJSONStringRoundTrip(f *testing.F) {
	for _, seed := range [][]byte{
		nil,
		[]byte("plain"),
		[]byte("&<>"),
		[]byte(`\`),
		[]byte(`\u0026`),
		[]byte(`\u002c`),
		[]byte(`\u002e`),
		{0x01},
		[]byte("\u2028"),
		{0xff},
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		value := string(data)
		got, gotErr := marshalJSONString(value)
		if !utf8.Valid(data) {
			if !errors.Is(gotErr, ErrJSONContract) || got != nil {
				t.Fatalf("marshalJSONString(invalid UTF-8) = (%q, %v), want (nil, %v)", got, gotErr, ErrJSONContract)
			}
			return
		}
		if gotErr != nil || !json.Valid(got) {
			t.Fatalf("marshalJSONString(valid UTF-8) = (%q, %v), want valid JSON/nil", got, gotErr)
		}
		roundTrip, gotUnquoteErr := strconv.Unquote(string(got))
		if gotUnquoteErr != nil || roundTrip != value {
			t.Fatalf(
				"independent string round trip = (%q, %v), want (%q, nil)",
				roundTrip,
				gotUnquoteErr,
				value,
			)
		}
	})
}

func TestStrictJSONPublicDiagnosticRatchet(t *testing.T) {
	t.Parallel()

	defaultLimits := DefaultStrictJSONLimits()
	cases := []struct {
		run         func() error
		name        string
		wantMessage string
	}{
		{
			name: "document byte limit identifies the exceeded resource",
			run: func() error {
				wire := bytes.Repeat([]byte{' '}, JSONDocumentMaximumBytes+1)
				_, err := DecodeStrictJSON[strictJSONTextRecord](wire, defaultLimits)
				return err
			},
			wantMessage: jsonDocumentLimitExceededErrorText,
		},
		{
			name: "invalid UTF8 identifies the encoding rule",
			run: func() error {
				_, err := DecodeStrictJSON[strictJSONTextRecord]([]byte{0xff}, defaultLimits)
				return err
			},
			wantMessage: jsonDocumentInvalidUTF8ErrorText,
		},
		{
			name: "lone high surrogate identifies the malformed escape",
			run: func() error {
				_, err := DecodeStrictJSON[strictJSONTextRecord](
					[]byte(`{"text":"\ud800"}`),
					defaultLimits,
				)
				return err
			},
			wantMessage: jsonUnpairedHighSurrogateErrorText,
		},
		{
			name: "lone low surrogate identifies the malformed escape",
			run: func() error {
				_, err := DecodeStrictJSON[strictJSONTextRecord](
					[]byte(`{"text":"\udfff"}`),
					defaultLimits,
				)
				return err
			},
			wantMessage: jsonUnpairedLowSurrogateErrorText,
		},
		{
			name: "nesting limit identifies the exceeded resource",
			run: func() error {
				_, err := DecodeStrictJSON[strictJSONTextRecord](
					strictJSONNestedArrays(JSONNestingDepthMaximum+1),
					defaultLimits,
				)
				return err
			},
			wantMessage: jsonNestingLimitExceededErrorText,
		},
		{
			name: "object field limit identifies the exceeded resource",
			run: func() error {
				_, err := DecodeStrictJSON[strictJSONTextRecord](
					strictJSONObject(JSONObjectFieldCountMaximum+1),
					defaultLimits,
				)
				return err
			},
			wantMessage: jsonObjectFieldLimitExceededErrorText,
		},
		{
			name: "array item limit identifies the exceeded resource",
			run: func() error {
				_, err := DecodeStrictJSON[strictJSONTextRecord](
					strictJSONArray(JSONArrayItemCountMaximum+1),
					defaultLimits,
				)
				return err
			},
			wantMessage: jsonArrayItemLimitExceededErrorText,
		},
		{
			name: "unstable producer identifies representation drift",
			run: func() error {
				_, err := EncodeValidatedJSON(
					unstableJSONRepresentation{},
					defaultLimits,
				)
				return err
			},
			wantMessage: jsonRepresentationUnstableErrorText,
		},
		{
			name: "decoded validation failure identifies the ownership boundary",
			run: func() error {
				_, err := DecodeStrictJSON[strictJSONTextRecord](
					[]byte(`{"text":""}`),
					defaultLimits,
				)
				return err
			},
			wantMessage: jsonDecodedValueInvalidErrorText,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotErr := tc.run()
			if !errors.Is(gotErr, ErrJSONContract) {
				t.Fatalf("strict JSON operation error = %v, want %v", gotErr, ErrJSONContract)
			}
			var gotDiagnostic jsonContractDiagnostic
			if !errors.As(gotErr, &gotDiagnostic) {
				t.Fatalf(
					"strict JSON operation error = %v, want typed diagnostic detail",
					gotErr,
				)
			}
			if gotMessage := gotDiagnostic.Error(); gotMessage != tc.wantMessage {
				t.Fatalf(
					"strict JSON operation diagnostic = %q, want detail %q",
					gotMessage,
					tc.wantMessage,
				)
			}
		})
	}
}

func TestStrictJSONStructuralLimitsDirectHelperHostileBoundaryTable(t *testing.T) {
	t.Parallel()

	custom := StrictJSONLimits{
		DocumentMaximumBytes: mustByteCountForTest(t, JSONDocumentMaximumBytes),
		NestingDepthMaximum:  2,
		ObjectFieldMaximum:   2,
		ArrayItemMaximum:     2,
	}
	if gotErr := custom.Validate(); gotErr != nil {
		t.Fatalf("custom StrictJSONLimits.Validate() error = %v, want nil", gotErr)
	}

	largeDocument := bytes.Repeat([]byte{' '}, JSONDocumentMaximumBytes-jsonArrayOverheadBytes)
	largeDocument = append(largeDocument, '[', ']')
	if _, gotErr := decodeStrictJSONStructure[json.RawMessage](largeDocument, custom); gotErr != nil {
		t.Fatalf("custom document limit decode error = %v, want nil", gotErr)
	}

	cases := []struct {
		name        string
		wire        []byte
		disposition boundaryDisposition
	}{
		{name: "scalar neutral to all container limits is accepted", wire: []byte(`0`), disposition: boundaryAccept},
		{name: "array one item below custom maximum is accepted", wire: []byte(`[0]`), disposition: boundaryAccept},
		{name: "array at custom item maximum is accepted", wire: []byte(`[0,0]`), disposition: boundaryAccept},
		{name: "array one item above custom maximum is rejected", wire: []byte(`[0,0,0]`)},
		{name: "array far above custom maximum is rejected", wire: strictJSONArray(JSONArrayItemCountMaximum)},
		{name: "object one field below custom maximum is accepted", wire: []byte(`{"a":0}`), disposition: boundaryAccept},
		{name: "object at custom field maximum is accepted", wire: []byte(`{"a":0,"b":0}`), disposition: boundaryAccept},
		{name: "object one field above custom maximum is rejected", wire: []byte(`{"a":0,"b":0,"c":0}`)},
		{name: "object far above custom maximum is rejected", wire: strictJSONObject(JSONObjectFieldCountMaximum)},
		{name: "nesting one level below custom maximum is accepted", wire: []byte(`[0]`), disposition: boundaryAccept},
		{name: "nesting at custom depth maximum is accepted", wire: []byte(`[[]]`), disposition: boundaryAccept},
		{name: "nesting one level above custom maximum is rejected", wire: []byte(`[[[]]]`)},
		{name: "nesting far above custom maximum is rejected", wire: strictJSONNestedArrays(JSONNestingDepthMaximum)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, gotErr := decodeStrictJSONStructure[json.RawMessage](tc.wire, custom)
			wantAccept := tc.disposition == boundaryAccept
			if (gotErr == nil) != wantAccept {
				t.Fatalf("decode with custom limits error = %v, want accept %t", gotErr, wantAccept)
			}
			if !wantAccept && !errors.Is(gotErr, ErrJSONContract) {
				t.Fatalf("decode with custom limits error = %v, want %v", gotErr, ErrJSONContract)
			}
		})
	}
}

func TestStrictJSONLimitsHostileBoundaryTable(t *testing.T) {
	t.Parallel()

	minimumDocument := mustByteCountForTest(t, 1)
	maximumDocument := mustByteCountForTest(t, JSONDocumentMaximumBytes)
	cases := []struct {
		wantErr error
		name    string
		limits  StrictJSONLimits
	}{
		{name: "minimum one values are accepted", limits: StrictJSONLimits{
			DocumentMaximumBytes: minimumDocument,
			NestingDepthMaximum:  1,
			ObjectFieldMaximum:   1,
			ArrayItemMaximum:     1,
		}},
		{name: "document one byte below maximum is accepted", limits: StrictJSONLimits{
			DocumentMaximumBytes: mustByteCountForTest(t, JSONDocumentMaximumBytes-1),
			NestingDepthMaximum:  JSONNestingDepthMaximum,
			ObjectFieldMaximum:   JSONObjectFieldCountMaximum,
			ArrayItemMaximum:     JSONArrayItemCountMaximum,
		}},
		{name: "document at maximum is accepted", limits: StrictJSONLimits{
			DocumentMaximumBytes: maximumDocument,
			NestingDepthMaximum:  JSONNestingDepthMaximum,
			ObjectFieldMaximum:   JSONObjectFieldCountMaximum,
			ArrayItemMaximum:     JSONArrayItemCountMaximum,
		}},
		{name: "document one byte above maximum is rejected", limits: StrictJSONLimits{
			DocumentMaximumBytes: mustByteCountForTest(t, JSONDocumentMaximumBytes+1),
			NestingDepthMaximum:  JSONNestingDepthMaximum,
			ObjectFieldMaximum:   JSONObjectFieldCountMaximum,
			ArrayItemMaximum:     JSONArrayItemCountMaximum,
		}, wantErr: ErrJSONContract},
		{name: "nesting one below maximum is accepted", limits: strictJSONLimitsForTest(maximumDocument, JSONNestingDepthMaximum-1, 1, 1)},
		{name: "nesting at maximum is accepted", limits: strictJSONLimitsForTest(maximumDocument, JSONNestingDepthMaximum, 1, 1)},
		{name: "nesting one above maximum is rejected", limits: strictJSONLimitsForTest(maximumDocument, JSONNestingDepthMaximum+1, 1, 1), wantErr: ErrJSONContract},
		{name: "object fields one below maximum are accepted", limits: strictJSONLimitsForTest(maximumDocument, 1, JSONObjectFieldCountMaximum-1, 1)},
		{name: "object fields at maximum are accepted", limits: strictJSONLimitsForTest(maximumDocument, 1, JSONObjectFieldCountMaximum, 1)},
		{name: "object fields one above maximum are rejected", limits: strictJSONLimitsForTest(maximumDocument, 1, JSONObjectFieldCountMaximum+1, 1), wantErr: ErrJSONContract},
		{name: "array items one below maximum are accepted", limits: strictJSONLimitsForTest(maximumDocument, 1, 1, JSONArrayItemCountMaximum-1)},
		{name: "array items at maximum are accepted", limits: strictJSONLimitsForTest(maximumDocument, 1, 1, JSONArrayItemCountMaximum)},
		{name: "array items one above maximum are rejected", limits: strictJSONLimitsForTest(maximumDocument, 1, 1, JSONArrayItemCountMaximum+1), wantErr: ErrJSONContract},
		{name: "zero limits are rejected", limits: StrictJSONLimits{}, wantErr: ErrJSONContract},
		{name: "zero document maximum is rejected", limits: strictJSONLimitsForTest(ByteCount{}, 1, 1, 1), wantErr: ErrJSONContract},
		{name: "zero nesting depth is rejected", limits: strictJSONLimitsForTest(minimumDocument, 0, 1, 1), wantErr: ErrJSONContract},
		{name: "maximum uint16 nesting is rejected", limits: strictJSONLimitsForTest(minimumDocument, math.MaxUint16, 1, 1), wantErr: ErrJSONContract},
		{name: "zero object field maximum is rejected", limits: strictJSONLimitsForTest(minimumDocument, 1, 0, 1), wantErr: ErrJSONContract},
		{name: "maximum uint16 object fields are rejected", limits: strictJSONLimitsForTest(minimumDocument, 1, math.MaxUint16, 1), wantErr: ErrJSONContract},
		{name: "zero array item maximum is rejected", limits: strictJSONLimitsForTest(minimumDocument, 1, 1, 0), wantErr: ErrJSONContract},
		{name: "maximum uint32 array items are rejected", limits: strictJSONLimitsForTest(minimumDocument, 1, 1, math.MaxUint32), wantErr: ErrJSONContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotErr := tc.limits.Validate()
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("StrictJSONLimits.Validate() error = %v, want %v", gotErr, tc.wantErr)
			}
		})
	}
}

func TestStrictJSONPublicDocumentLimitBoundary(t *testing.T) {
	t.Parallel()

	record := strictJSONTextRecord{Text: strings.Repeat("x", JSONDocumentMaximumBytes-128)}
	stdlibWire, stdlibErr := json.Marshal(record)
	if stdlibErr != nil {
		t.Fatalf("json.Marshal(strictJSONTextRecord) error = %v, want nil", stdlibErr)
	}
	exactDocumentBytes := uint64(len(stdlibWire))
	cases := []struct {
		name        string
		limitBytes  uint64
		disposition boundaryDisposition
	}{
		{name: "one byte below encoded length rejects the typed document", limitBytes: exactDocumentBytes - 1},
		{name: "exact encoded length accepts the typed document", limitBytes: exactDocumentBytes, disposition: boundaryAccept},
		{name: "one byte above encoded length accepts the typed document", limitBytes: exactDocumentBytes + 1, disposition: boundaryAccept},
		{name: "global maximum accepts the typed document", limitBytes: JSONDocumentMaximumBytes, disposition: boundaryAccept},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			limits := DefaultStrictJSONLimits()
			limits.DocumentMaximumBytes = mustByteCountForTest(t, tc.limitBytes)
			gotWire, gotEncodeErr := EncodeValidatedJSON(record, limits)
			wantAccept := tc.disposition == boundaryAccept
			if (gotEncodeErr == nil) != wantAccept {
				t.Fatalf("EncodeValidatedJSON() = (%d bytes, %v), want accept %t", len(gotWire), gotEncodeErr, wantAccept)
			}
			if !wantAccept {
				if !errors.Is(gotEncodeErr, ErrJSONContract) {
					t.Fatalf("EncodeValidatedJSON() error = %v, want %v", gotEncodeErr, ErrJSONContract)
				}
				if gotWire != nil {
					t.Fatalf("EncodeValidatedJSON() wire length = %d, want nil", len(gotWire))
				}
				return
			}
			if !bytes.Equal(gotWire, stdlibWire) {
				t.Fatalf("EncodeValidatedJSON() wire length = %d, want canonical length %d", len(gotWire), len(stdlibWire))
			}
			gotRecord, gotDecodeErr := DecodeStrictJSON[strictJSONTextRecord](gotWire, limits)
			if gotDecodeErr != nil || gotRecord.Text != record.Text {
				t.Fatalf(
					"DecodeStrictJSON() = (text length %d, %v), want (text length %d, nil)",
					len(gotRecord.Text),
					gotDecodeErr,
					len(record.Text),
				)
			}
		})
	}
}

func BenchmarkEncodeValidatedJSONAbsolutePath(b *testing.B) {
	path := validatedJSONEncodeRatchetPath(b)
	limits := DefaultStrictJSONLimits()
	var gotWire []byte
	var gotErr error
	gotAllocations := testing.AllocsPerRun(
		validatedJSONEncodeAllocationSamples,
		func() {
			gotWire, gotErr = EncodeValidatedJSON(path, limits)
		},
	)
	if gotErr != nil {
		b.Fatalf("EncodeValidatedJSON(allocation ratchet path) error = %v, want nil", gotErr)
	}
	if !json.Valid(gotWire) {
		b.Fatalf("EncodeValidatedJSON(allocation ratchet path) wire = %q, want valid JSON", gotWire)
	}
	if gotAllocations > validatedJSONEncodeAllocationMaximum {
		b.Fatalf(
			"EncodeValidatedJSON() allocations = %.0f, want <= %d",
			gotAllocations,
			validatedJSONEncodeAllocationMaximum,
		)
	}
	gotBytes, gotBytesErr := validatedJSONEncodeBytesPerRun(path, limits)
	if gotBytesErr != nil {
		b.Fatalf("validatedJSONEncodeBytesPerRun() error = %v, want nil", gotBytesErr)
	}
	if gotBytes > validatedJSONEncodeBytesMaximum {
		b.Fatalf(
			"EncodeValidatedJSON() allocated bytes/run = %d, want <= %d",
			gotBytes,
			validatedJSONEncodeBytesMaximum,
		)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if _, err := EncodeValidatedJSON(path, limits); err != nil {
			b.Fatalf("EncodeValidatedJSON() error = %v, want nil", err)
		}
	}
	b.ReportMetric(float64(gotBytes), "ratchet-B/op")
}

func BenchmarkRejectDuplicateJSONFieldsMaximum(b *testing.B) {
	document := strictJSONObject(JSONObjectFieldCountMaximum)
	limits := DefaultStrictJSONLimits()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if err := rejectDuplicateJSONFields(document, limits); err != nil {
			b.Fatalf("rejectDuplicateJSONFields(maximum) error = %v, want nil", err)
		}
	}
}

func BenchmarkRejectDuplicateJSONFieldsGlobalMaximumLongSharedPrefix(b *testing.B) {
	document := strictJSONObjectWithSharedKeyPrefix(
		JSONObjectFieldCountMaximum,
		strictJSONHostileSharedPrefixBytes,
	)
	if len(document) < JSONDocumentMaximumBytes-strictJSONHostileMaximumSlackBytes ||
		len(document) > JSONDocumentMaximumBytes {
		b.Fatalf(
			"hostile shared-prefix document bytes = %d, want [%d, %d]",
			len(document),
			JSONDocumentMaximumBytes-strictJSONHostileMaximumSlackBytes,
			JSONDocumentMaximumBytes,
		)
	}
	documentMaximum, gotLimitErr := NewByteCount(JSONDocumentMaximumBytes)
	if gotLimitErr != nil {
		b.Fatalf(
			"NewByteCount(JSONDocumentMaximumBytes) error = %v, want nil",
			gotLimitErr,
		)
	}
	limits := DefaultStrictJSONLimits()
	limits.DocumentMaximumBytes = documentMaximum
	limits.ObjectFieldMaximum = JSONObjectFieldCountMaximum
	limits.ArrayItemMaximum = JSONArrayItemCountMaximum
	b.SetBytes(int64(len(document)))
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if err := rejectDuplicateJSONFields(document, limits); err != nil {
			b.Fatalf(
				"rejectDuplicateJSONFields(global maximum long shared prefix) error = %v, want nil",
				err,
			)
		}
	}
}

func BenchmarkDecodeStrictJSONAdvertisedMaximumComposition(b *testing.B) {
	object := strictJSONObjectWithSharedKeyPrefix(
		JSONObjectFieldCountMaximum,
		strictJSONHostileComposedPrefixBytes,
	)
	objectCount := (JSONDocumentMaximumBytes - jsonArrayOverheadBytes + 1) /
		(len(object) + 1)
	document := strictJSONArrayOfRepeatedValue(
		object,
		objectCount,
	)
	if len(document) < JSONDocumentMaximumBytes-strictJSONHostileMaximumSlackBytes ||
		len(document) > JSONDocumentMaximumBytes {
		b.Fatalf(
			"hostile composed document bytes = %d, want [%d, %d]",
			len(document),
			JSONDocumentMaximumBytes-strictJSONHostileMaximumSlackBytes,
			JSONDocumentMaximumBytes,
		)
	}
	documentMaximum, gotLimitErr := NewByteCount(JSONDocumentMaximumBytes)
	if gotLimitErr != nil {
		b.Fatalf(
			"NewByteCount(JSONDocumentMaximumBytes) error = %v, want nil",
			gotLimitErr,
		)
	}
	limits := DefaultStrictJSONLimits()
	limits.DocumentMaximumBytes = documentMaximum
	limits.ObjectFieldMaximum = JSONObjectFieldCountMaximum
	limits.ArrayItemMaximum = JSONArrayItemCountMaximum
	var got strictJSONBenchmarkDocument
	var gotErr error
	gotAllocations := testing.AllocsPerRun(1, func() {
		got, gotErr = DecodeStrictJSON[strictJSONBenchmarkDocument](document, limits)
	})
	if gotErr != nil || !got.decoded {
		b.Fatalf(
			"DecodeStrictJSON(allocation ratchet) = (%v, %v), want (decoded, nil)",
			got,
			gotErr,
		)
	}
	if gotAllocations > strictJSONDecodeAllocationMaximum {
		b.Fatalf(
			"DecodeStrictJSON() allocations = %.0f, want <= %d",
			gotAllocations,
			strictJSONDecodeAllocationMaximum,
		)
	}
	gotBytes, gotBytesErr := strictJSONDecodeBytesPerRun(document, limits)
	if gotBytesErr != nil {
		b.Fatalf("strictJSONDecodeBytesPerRun() error = %v, want nil", gotBytesErr)
	}
	if gotBytes > strictJSONDecodeBytesMaximum {
		b.Fatalf(
			"DecodeStrictJSON() allocated bytes/run = %d, want <= %d",
			gotBytes,
			strictJSONDecodeBytesMaximum,
		)
	}
	b.SetBytes(int64(len(document)))
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		got, err := DecodeStrictJSON[strictJSONBenchmarkDocument](document, limits)
		if err != nil || !got.decoded {
			b.Fatalf(
				"DecodeStrictJSON(advertised maximum composition) = (%v, %v), want (decoded, nil)",
				got,
				err,
			)
		}
	}
	b.ReportMetric(float64(gotBytes), "ratchet-B/op")
}

func strictJSONDecodeBytesPerRun(
	document []byte,
	limits StrictJSONLimits,
) (uint64, error) {
	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)
	got, err := DecodeStrictJSON[strictJSONBenchmarkDocument](document, limits)
	if err != nil {
		return 0, err
	}
	if !got.decoded {
		return 0, ErrPrimitiveContract
	}
	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	return after.TotalAlloc - before.TotalAlloc, nil
}

func BenchmarkJSONMarshalAbsolutePath(b *testing.B) {
	path := validatedJSONEncodeRatchetPath(b)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if _, err := json.Marshal(path); err != nil {
			b.Fatalf("json.Marshal() error = %v, want nil", err)
		}
	}
}

func validatedJSONEncodeBytesPerRun(
	path AbsolutePath,
	limits StrictJSONLimits,
) (uint64, error) {
	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)
	for range validatedJSONEncodeMemorySamples {
		if _, err := EncodeValidatedJSON(path, limits); err != nil {
			return 0, err
		}
	}
	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	return (after.TotalAlloc - before.TotalAlloc) / validatedJSONEncodeMemorySamples, nil
}

func strictJSONLimitsForTest(
	documentMaximum ByteCount,
	nestingMaximum uint16,
	objectMaximum uint16,
	arrayMaximum uint32,
) StrictJSONLimits {
	return StrictJSONLimits{
		DocumentMaximumBytes: documentMaximum,
		NestingDepthMaximum:  nestingMaximum,
		ObjectFieldMaximum:   objectMaximum,
		ArrayItemMaximum:     arrayMaximum,
	}
}

func mustByteCountForTest(t *testing.T, value uint64) ByteCount {
	t.Helper()

	count, err := NewByteCount(value)
	if err != nil {
		t.Fatalf("NewByteCount(%d) error = %v, want nil", value, err)
	}
	return count
}

func validatedJSONEncodeRatchetPath(tb testing.TB) AbsolutePath {
	tb.Helper()

	component, componentErr := ParsePathComponent(validatedJSONEncodeRatchetComponent)
	if componentErr != nil {
		tb.Fatalf(
			"ParsePathComponent(%q) error = %v, want nil",
			validatedJSONEncodeRatchetComponent,
			componentErr,
		)
	}
	root, rootErr := ParseAbsolutePath(string(filepath.Separator))
	if rootErr != nil {
		tb.Fatalf("ParseAbsolutePath(root) error = %v, want nil", rootErr)
	}
	path, pathErr := root.Join(component)
	if pathErr != nil {
		tb.Fatalf("root.Join(%q) error = %v, want nil", component, pathErr)
	}
	return path
}

func decodeStrictJSONStructure[T any](data []byte, limits StrictJSONLimits) (T, error) {
	var zero T
	if err := limits.Validate(); err != nil {
		return zero, err
	}
	return decodeStrictJSONStructureValidatedLimits[T](data, limits)
}

func strictJSONGlobalObjectFieldLimits() StrictJSONLimits {
	limits := DefaultStrictJSONLimits()
	limits.ObjectFieldMaximum = JSONObjectFieldCountMaximum
	return limits
}

func strictJSONObject(fieldCount int) []byte {
	var document strings.Builder
	document.Grow(fieldCount*8 + jsonObjectOverheadBytes)
	document.WriteByte('{')
	for index := range fieldCount {
		if index > 0 {
			document.WriteByte(',')
		}
		document.WriteString(strconv.Quote(strconv.Itoa(index)))
		document.WriteString(":0")
	}
	document.WriteByte('}')
	return []byte(document.String())
}

func strictJSONObjectWithFinalDuplicate(fieldCount int) []byte {
	if fieldCount < 2 {
		return nil
	}
	var document strings.Builder
	document.Grow(fieldCount*8 + jsonObjectOverheadBytes)
	document.WriteByte('{')
	for index := range fieldCount {
		if index > 0 {
			document.WriteByte(',')
		}
		key := strconv.Itoa(index)
		if index == fieldCount-1 {
			key = "0"
		}
		document.WriteString(strconv.Quote(key))
		document.WriteString(":0")
	}
	document.WriteByte('}')
	return []byte(document.String())
}

func strictJSONObjectWithSharedKeyPrefix(fieldCount int, prefixBytes int) []byte {
	prefix := strings.Repeat("a", prefixBytes)
	var document strings.Builder
	document.Grow(fieldCount*(prefixBytes+16) + jsonObjectOverheadBytes)
	document.WriteByte('{')
	for index := range fieldCount {
		if index > 0 {
			document.WriteByte(',')
		}
		document.WriteByte('"')
		document.WriteString(prefix)
		document.WriteString(strconv.Itoa(index))
		document.WriteString(`":0`)
	}
	document.WriteByte('}')
	return []byte(document.String())
}

func strictJSONArrayOfRepeatedValue(value []byte, itemCount int) []byte {
	document := make([]byte, 0, itemCount*(len(value)+1)+jsonArrayOverheadBytes)
	document = append(document, '[')
	for index := range itemCount {
		if index > 0 {
			document = append(document, ',')
		}
		document = append(document, value...)
	}
	document = append(document, ']')
	return document
}

func strictJSONArray(itemCount int) []byte {
	if itemCount == 0 {
		return []byte("[]")
	}
	document := make([]byte, 0, itemCount*2+jsonArrayOverheadBytes)
	document = append(document, '[')
	for index := range itemCount {
		if index > 0 {
			document = append(document, ',')
		}
		document = append(document, '0')
	}
	document = append(document, ']')
	return document
}

func strictJSONNestedArrays(depth int) []byte {
	document := make([]byte, 0, depth*2+1)
	document = append(document, bytes.Repeat([]byte{'['}, depth)...)
	document = append(document, '0')
	document = append(document, bytes.Repeat([]byte{']'}, depth)...)
	return document
}
