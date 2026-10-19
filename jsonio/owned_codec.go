package jsonio

import (
	"context"
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"errors"
	"io"
	"reflect"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// MarshalValidated emits one compact validated value with caller-owned
// allocation. Go owns JSON, Unicode, duplicate names and deterministic output.
// This whole-value door imposes no stream quota. Prefer ObjectEncoder or
// EncodeObject when the output can remain a sequence of typed records.
// PointerNullRefused checks the completed native projection for emitted null;
// omission remains Go's decision and publishes no null token.
// Source: https://pkg.go.dev/encoding/json/v2#Marshal
func MarshalValidated[Document core.Validatable](document Document, policy OwnedCodecPolicy) ([]byte, error) {
	if err := validateOwnedEncoding(document, policy); err != nil {
		return nil, err
	}
	data, err := jsonv2.Marshal(document, jsonv2.Deterministic(true))
	if err != nil {
		return nil, jsonOperationError(err)
	}
	if !jsontext.Value(data).IsValid() || jsontext.Value(data).Kind() == jsontext.KindNull {
		return nil, core.ErrJSONContract
	}
	if policy.PointerNulls == PointerNullRefused {
		if err := refuseOwnedNullProjection(data); err != nil {
			return nil, err
		}
	}
	return data, nil
}

// Go does not call MarshalToFunc for nil pointers. Inspect its complete
// projection with Go's token cursor instead of reproducing field/tag rules;
// omitted pointers emit no token and therefore remain omitted.
// Source: https://pkg.go.dev/encoding/json/v2#MarshalToFunc
func refuseOwnedNullProjection(data []byte) error {
	decoder := borrowedBytesDecoder(data)
	for {
		token, err := decoder.ReadToken()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return jsonOperationError(err)
		}
		if token.Kind() == jsontext.KindNull {
			return core.ErrJSONContract
		}
	}
}

func ownedMarshalGuards(ctx context.Context) *jsonv2.Marshalers {
	return jsonv2.MarshalToFunc[any](func(_ *jsontext.Encoder, value any) error {
		// The wildcard is Go's metadata hook, never a retained payload bag.
		// Opaque method internals remain their owner's explicit agreement.
		if ctx != nil {
			if err := contextstate.Validate(ctx); err != nil {
				return &nativeJSONError{cause: err}
			}
		}
		return errors.ErrUnsupported
	})
}

func validateOwnedEncoding[Document core.Validatable](document Document, policy OwnedCodecPolicy) error {
	if err := validateOwnedCodecType[Document](policy); err != nil {
		return err
	}
	if value := reflect.ValueOf(document); value.Kind() == reflect.Pointer && value.IsNil() {
		return core.ErrJSONContract
	}
	if err := document.Validate(); err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	return nil
}

// DecodeOwned admits one complete caller-owned JSON value. Native grammar,
// duplicate-name and UTF-8 checks run before an explicitly admitted custom
// codec can see bytes. Failure publishes exactly zero; the input is borrowed.
// Native custom methods own admission within their private projection, so
// Members and PointerNulls govern fields decoded by Go's declared-field codec.
// Source: https://pkg.go.dev/encoding/json/v2#Unmarshal
func DecodeOwned[Document core.Validatable](data []byte, policy OwnedCodecPolicy) (Document, error) {
	var zero Document
	if err := validateOwnedCodecType[Document](policy); err != nil {
		return zero, err
	}
	value := jsontext.Value(data)
	if !value.IsValid() || value.Kind() == jsontext.KindNull {
		return zero, core.ErrJSONContract
	}
	guards := ownedNullRefusalGuard
	if policy.PointerNulls == PointerNullPermitted {
		guards = ownedNullPointerGuard
	}
	var document Document
	if err := jsonv2.Unmarshal(data, &document, jsonv2.RejectUnknownMembers(policy.Members == MembersDeclaredOnly), jsonv2.WithUnmarshalers(guards)); err != nil {
		return zero, jsonOperationError(err)
	}
	if pointer := reflect.ValueOf(document); pointer.Kind() == reflect.Pointer && pointer.IsNil() {
		return zero, core.ErrJSONContract
	}
	if err := document.Validate(); err != nil {
		return zero, errors.Join(core.ErrJSONContract, err)
	}
	return document, nil
}

// The two pointer-null arms are stateless hook lists shared by every decode,
// so Go's concurrency-safe type cache is built once per arm, not per value.
var (
	ownedNullRefusalGuard = jsonv2.UnmarshalFromFunc[any](ownedNullGuard(PointerNullRefused))
	ownedNullPointerGuard = jsonv2.UnmarshalFromFunc[any](ownedNullGuard(PointerNullPermitted))
)

func ownedNullGuard(nulls PointerNullPolicy) func(*jsontext.Decoder, any) error {
	return func(decoder *jsontext.Decoder, target any) error {
		// Go lends a non-nil pointer to the declared destination. A pointer
		// field is therefore observed as **Field, not as an untyped world.
		if decoder.PeekKind() != jsontext.KindNull {
			return errors.ErrUnsupported
		}
		typeOf := reflect.TypeOf(target)
		if nulls == PointerNullPermitted && typeOf != nil && typeOf.Kind() == reflect.Pointer && typeOf.Elem().Kind() == reflect.Pointer {
			return errors.ErrUnsupported
		}
		return core.ErrJSONContract
	}
}

func validateOwnedCodecType[Document any](policy OwnedCodecPolicy) error {
	if err := policy.Validate(); err != nil {
		return err
	}
	if policy.Root == RootDeclaredFields {
		return validateObjectType[Document]()
	}
	root := reflect.TypeFor[Document]()
	if root.Kind() == reflect.Pointer {
		root = root.Elem()
	}
	switch root.Kind() {
	case reflect.Struct, reflect.String, reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Float32, reflect.Float64, reflect.Array, reflect.Slice:
		return nil
	default:
		return core.ErrJSONContract
	}
}
