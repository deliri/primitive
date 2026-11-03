package jsonio

import (
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"reflect"

	"github.com/deliri/primitive/v2026/core"
)

// NativeCodec lends one bidirectional, type-directed whole-value projection
// for an owner type that has no JSON methods of its own, or whose methods the
// document owner deliberately replaces. The opaque native hooks retain no
// document; a zero codec is invalid.
type NativeCodec struct {
	marshal   *jsonv2.Marshalers
	unmarshal *jsonv2.Unmarshalers
}

func (c NativeCodec) Validate() error {
	if c.marshal == nil || c.unmarshal == nil {
		return core.ErrJSONContract
	}
	return nil
}

// ProjectNative binds an owner to a hook-free projection in both directions.
// Encoding projects the owner and spells the projection through Go; decoding
// reads the projection with declared-only members and restores the owner.
// A null never reaches restore: it is refused before any projection decode,
// while a null for an actual pointer to the owner follows the outer door's
// pointer-null policy. Owner refusals from project or restore survive as the
// native cause. Like Go's own lent marshalers, the codec takes precedence over
// the owner's methods.
func ProjectNative[Owner, Projection any](project func(Owner) (Projection, error), restore func(Projection) (Owner, error)) (NativeCodec, error) {
	if project == nil || restore == nil {
		return NativeCodec{}, core.ErrJSONContract
	}
	owner, projection := indirectType(reflect.TypeFor[Owner]()), indirectType(reflect.TypeFor[Projection]())
	if owner == projection || !admittedCodecKind(projection) || nativeCodecHooks(projection) {
		return NativeCodec{}, core.ErrJSONContract
	}
	marshal := jsonv2.MarshalToFunc[Owner](func(encoder *jsontext.Encoder, value Owner) error {
		projected, err := project(value)
		if err != nil {
			return err
		}
		return jsonv2.MarshalEncode(encoder, projected, jsonv2.Deterministic(true))
	})
	unmarshal := jsonv2.UnmarshalFromFunc[*Owner](func(decoder *jsontext.Decoder, target *Owner) error {
		if decoder.PeekKind() == jsontext.KindNull {
			return core.ErrJSONContract
		}
		var projected Projection
		if err := jsonv2.UnmarshalDecode(decoder, &projected, jsonv2.RejectUnknownMembers(true)); err != nil {
			return err
		}
		value, err := restore(projected)
		if err != nil {
			return err
		}
		*target = value
		return nil
	})
	return NativeCodec{marshal: marshal, unmarshal: unmarshal}, nil
}

// MarshalValidatedWith is MarshalValidated with lent native codecs. Members,
// pointer nulls, root policy, validation and refusal remain identical; with no
// codecs it is the same door.
func MarshalValidatedWith[Document core.Validatable](document Document, policy OwnedCodecPolicy, codecs ...NativeCodec) ([]byte, error) {
	return marshalValidated(document, policy, codecs)
}

// DecodeOwnedWith is DecodeOwned with lent native codecs. The null guards keep
// precedence, so a null for a projected non-pointer member refuses under both
// pointer-null arms before any restore runs.
func DecodeOwnedWith[Document core.Validatable](data []byte, policy OwnedCodecPolicy, codecs ...NativeCodec) (Document, error) {
	return decodeOwned[Document](data, policy, codecs)
}

func nativeMarshalers(codecs []NativeCodec) ([]jsonv2.Options, error) {
	if len(codecs) == 0 {
		return nil, nil
	}
	lent := make([]*jsonv2.Marshalers, 0, len(codecs))
	for _, codec := range codecs {
		if err := codec.Validate(); err != nil {
			return nil, err
		}
		lent = append(lent, codec.marshal)
	}
	return []jsonv2.Options{jsonv2.WithMarshalers(jsonv2.JoinMarshalers(lent...))}, nil
}

func nativeUnmarshalers(guards *jsonv2.Unmarshalers, codecs []NativeCodec) (*jsonv2.Unmarshalers, error) {
	if len(codecs) == 0 {
		return guards, nil
	}
	lent := make([]*jsonv2.Unmarshalers, 0, len(codecs)+1)
	lent = append(lent, guards)
	for _, codec := range codecs {
		if err := codec.Validate(); err != nil {
			return nil, err
		}
		lent = append(lent, codec.unmarshal)
	}
	return jsonv2.JoinUnmarshalers(lent...), nil
}

func indirectType(root reflect.Type) reflect.Type {
	for root.Kind() == reflect.Pointer {
		root = root.Elem()
	}
	return root
}

// admittedCodecKind is the closed set a whole-value owned codec may spell.
func admittedCodecKind(root reflect.Type) bool {
	switch root.Kind() {
	case reflect.Struct, reflect.String, reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Float32, reflect.Float64, reflect.Array, reflect.Slice:
		return true
	default:
		return false
	}
}

// nativeCodecHooks reports whether a type or its pointer owns a JSON method.
func nativeCodecHooks(root reflect.Type) bool {
	for _, candidate := range [...]reflect.Type{root, reflect.PointerTo(root)} {
		for _, hook := range [...]reflect.Type{
			reflect.TypeFor[interface{ MarshalJSON() ([]byte, error) }](), reflect.TypeFor[interface{ UnmarshalJSON([]byte) error }](),
			reflect.TypeFor[jsonv2.MarshalerTo](), reflect.TypeFor[jsonv2.UnmarshalerFrom](),
		} {
			if candidate.Implements(hook) {
				return true
			}
		}
	}
	return false
}

var _ core.Validatable = NativeCodec{}
