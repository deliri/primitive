package receipt

import (
	"encoding/hex"
	json "encoding/json/v2"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

type lifecycleJSONIdentity interface {
	core.ValidatedJSONMarshaler
	String() string
}

func TestLifecycleIdentityLayerTriad(t *testing.T) {
	t.Parallel()

	type identityCase struct {
		construct func([LifecycleIdentityBytes]byte) (core.Validatable, error)
		parse     func(string) (core.Validatable, error)
		name      string
	}
	cases := []identityCase{
		{
			name: "account",
			construct: func(value [LifecycleIdentityBytes]byte) (core.Validatable, error) {
				return NewAccountIdentity(value)
			},
			parse: func(value string) (core.Validatable, error) {
				return ParseAccountIdentity(value)
			},
		},
		{
			name: "submission",
			construct: func(value [LifecycleIdentityBytes]byte) (core.Validatable, error) {
				return NewSubmissionIdentity(value)
			},
			parse: func(value string) (core.Validatable, error) {
				return ParseSubmissionIdentity(value)
			},
		},
		{
			name: "object",
			construct: func(value [LifecycleIdentityBytes]byte) (core.Validatable, error) {
				return NewObjectIdentity(value)
			},
			parse: func(value string) (core.Validatable, error) {
				return ParseObjectIdentity(value)
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var zero [LifecycleIdentityBytes]byte
			if _, gotErr := tc.construct(zero); !errors.Is(gotErr, core.ErrLifecycleIdentityContract) ||
				!errors.Is(gotErr, core.ErrReceiptContract) {
				t.Fatalf("construct(zero) error = %v, want lifecycle and Receipt identities", gotErr)
			}
			lowest := zero
			lowest[len(lowest)-1] = 1
			got, gotErr := tc.construct(lowest)
			if gotErr != nil || got.Validate() != nil {
				t.Fatalf("construct(lowest nonzero) = (%v, %v), want valid and nil", got, gotErr)
			}
			text := strings.Repeat("0", LifecycleIdentityHexBytes-1) + "1"
			parsed, gotParseErr := tc.parse(text)
			if gotParseErr != nil || parsed.Validate() != nil {
				t.Fatalf("parse(lowest canonical) = (%v, %v), want valid and nil", parsed, gotParseErr)
			}
			for _, hostile := range []string{
				"",
				text[:len(text)-1],
				text + "0",
				strings.Repeat("0", LifecycleIdentityHexBytes-1) + "A",
				strings.Repeat("z", LifecycleIdentityHexBytes),
				string([]byte{0xff}),
			} {
				if _, gotErr := tc.parse(hostile); !errors.Is(gotErr, core.ErrLifecycleIdentityContract) ||
					!errors.Is(gotErr, core.ErrReceiptContract) {
					t.Fatalf("parse(%q) error = %v, want lifecycle and Receipt identities", hostile, gotErr)
				}
			}
		})
	}

	_, err := ParseAccountIdentity(strings.Repeat("z", LifecycleIdentityHexBytes))
	if _, ok := errors.AsType[hex.InvalidByteError](err); !ok {
		t.Fatalf("ParseAccountIdentity(non-hex) error = %v, want native hex.InvalidByteError", err)
	}
}

func TestLifecycleIdentityNominalAndJSONBoundaries(t *testing.T) {
	t.Parallel()

	types := []reflect.Type{
		reflect.TypeFor[AccountIdentity](),
		reflect.TypeFor[SubmissionIdentity](),
		reflect.TypeFor[ObjectIdentity](),
	}
	for left := range types {
		for right := range types {
			if left != right && types[left].ConvertibleTo(types[right]) {
				t.Fatalf("%s converts to %s, want nominal separation", types[left], types[right])
			}
		}
	}

	var raw [LifecycleIdentityBytes]byte
	for index := range raw {
		raw[index] = byte(index + 1)
	}

	type identityJSONCase struct {
		construct    func([LifecycleIdentityBytes]byte) (lifecycleJSONIdentity, error)
		nilUnmarshal func([]byte) error
		unmarshal    func(lifecycleJSONIdentity, []byte) (lifecycleJSONIdentity, error)
		name         string
	}
	cases := []identityJSONCase{
		{
			name: "account",
			construct: func(value [LifecycleIdentityBytes]byte) (lifecycleJSONIdentity, error) {
				return NewAccountIdentity(value)
			},
			unmarshal: func(seed lifecycleJSONIdentity, data []byte) (lifecycleJSONIdentity, error) {
				value := seed.(AccountIdentity)
				return value, value.UnmarshalJSON(data)
			},
			nilUnmarshal: func(data []byte) error {
				var value *AccountIdentity
				return value.UnmarshalJSON(data)
			},
		},
		{
			name: "submission",
			construct: func(value [LifecycleIdentityBytes]byte) (lifecycleJSONIdentity, error) {
				return NewSubmissionIdentity(value)
			},
			unmarshal: func(seed lifecycleJSONIdentity, data []byte) (lifecycleJSONIdentity, error) {
				value := seed.(SubmissionIdentity)
				return value, value.UnmarshalJSON(data)
			},
			nilUnmarshal: func(data []byte) error {
				var value *SubmissionIdentity
				return value.UnmarshalJSON(data)
			},
		},
		{
			name: "object",
			construct: func(value [LifecycleIdentityBytes]byte) (lifecycleJSONIdentity, error) {
				return NewObjectIdentity(value)
			},
			unmarshal: func(seed lifecycleJSONIdentity, data []byte) (lifecycleJSONIdentity, error) {
				value := seed.(ObjectIdentity)
				return value, value.UnmarshalJSON(data)
			},
			nilUnmarshal: func(data []byte) error {
				var value *ObjectIdentity
				return value.UnmarshalJSON(data)
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			original, err := tc.construct(raw)
			if err != nil {
				t.Fatalf("construct() error = %v, want nil", err)
			}
			if original.String() != hex.EncodeToString(raw[:]) {
				t.Fatalf("String() = %q, want canonical identity", original.String())
			}
			canonical, err := json.Marshal(original)
			if err != nil {
				t.Fatalf("json.Marshal() error = %v, want nil", err)
			}
			if gotErr := tc.nilUnmarshal(canonical); !errors.Is(gotErr, core.ErrJSONContract) ||
				!errors.Is(gotErr, core.ErrLifecycleIdentityContract) ||
				!errors.Is(gotErr, core.ErrReceiptContract) {
				t.Fatalf("nil receiver UnmarshalJSON() error = %v, want JSON+lifecycle+Receipt identities", gotErr)
			}
			decoded, gotErr := tc.unmarshal(mustDifferentLifecycleIdentity(t, tc.construct, raw), canonical)
			if gotErr != nil || decoded != original {
				t.Fatalf("UnmarshalJSON(canonical) = (%v, %v), want (%v, nil)", decoded, gotErr, original)
			}
			for _, data := range [][]byte{
				nil,
				[]byte("null"),
				canonical[:len(canonical)-1],
				[]byte(`"` + strings.ToUpper(original.String()) + `"`),
				[]byte{'"', 0xff, '"'},
			} {
				receiver, gotErr := tc.unmarshal(original, data)
				if !errors.Is(gotErr, core.ErrJSONContract) ||
					!errors.Is(gotErr, core.ErrLifecycleIdentityContract) ||
					!errors.Is(gotErr, core.ErrReceiptContract) ||
					receiver != original {
					t.Fatalf("UnmarshalJSON(%q) = (%v, %v), want preserved receiver and JSON+lifecycle identities", data, receiver, gotErr)
				}
			}
		})
	}
}

func mustDifferentLifecycleIdentity(
	t *testing.T,
	construct func([LifecycleIdentityBytes]byte) (lifecycleJSONIdentity, error),
	raw [LifecycleIdentityBytes]byte,
) lifecycleJSONIdentity {
	t.Helper()

	raw[0]++
	value, err := construct(raw)
	if err != nil {
		t.Fatalf("construct(different identity) error = %v, want nil", err)
	}
	return value
}
