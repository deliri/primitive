package jsonio

import (
	"context"
	"encoding"
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"errors"
	"io"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"unicode/utf16"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// CanonicalProjectionCodec is a compiler-typed projection registered with Go.
// The opaque native codec retains no population or encoded document.
type CanonicalProjectionCodec struct{ native *jsonv2.Marshalers }

// ProjectCanonical lends an owning type's concrete wire projection to Go's
// streaming encoder. It replaces that type's whole-value custom marshaler.
func ProjectCanonical[Owner, Projection any](project func(Owner) (Projection, error)) (CanonicalProjectionCodec, error) {
	if project == nil {
		return CanonicalProjectionCodec{}, core.ErrJSONContract
	}
	owner, projection := reflect.TypeFor[Owner](), reflect.TypeFor[Projection]()
	for owner.Kind() == reflect.Pointer {
		owner = owner.Elem()
	}
	for projection.Kind() == reflect.Pointer {
		projection = projection.Elem()
	}
	if owner == projection {
		return CanonicalProjectionCodec{}, core.ErrJSONContract
	}
	native := jsonv2.MarshalToFunc[Owner](func(encoder *jsontext.Encoder, value Owner) error {
		projection, err := project(value)
		if err != nil {
			return err
		}
		return jsonv2.MarshalEncode(encoder, projection)
	})
	return CanonicalProjectionCodec{native: native}, nil
}

// WriteCanonicalProjection streams concrete declared fields and array elements
// without a top-level LF. Go owns field encoding, omission, grammar, escaping,
// and numeric spelling. Primitive orders the declared field metadata by UTF-16
// name; it never collects array elements. Explicit projections can replace an
// owner's whole-value custom codec. Other custom methods retain their own
// value's bytes, canonicalized by Go before publication.
//
// Declared fields require explicit, unquoted JSON names. Untagged embedded
// structs are flattened; conflicting names, inline/unknown fallback fields,
// untagged ordinary fields, and maps are refused. These restrictions keep one
// unambiguous declared schema rather than reproducing Go's field resolution.
// Memory follows declaration size, open depth and the largest native/custom
// value; a caller's custom marshaler can still retain its own complete value.
// Declared containers obey Core's nesting maximum. Opaque custom methods keep
// their own depth and allocation contracts.
func WriteCanonicalProjection[Document any](ctx context.Context, destination io.Writer, document Document, policy OwnedCodecPolicy, codecs ...CanonicalProjectionCodec) error {
	if err := errors.Join(contextstate.Validate(ctx), validateOwnedCodecType[Document](policy)); err != nil {
		return err
	}
	if core.WriterIsNil(destination) {
		return core.ErrJSONContract
	}
	if value := reflect.ValueOf(document); value.Kind() == reflect.Pointer && value.IsNil() {
		return core.ErrJSONContract
	}
	marshalers := make([]*jsonv2.Marshalers, 0, len(codecs)+1)
	for _, codec := range codecs {
		if codec.native == nil {
			return core.ErrJSONContract
		}
		marshalers = append(marshalers, codec.native)
	}
	marshalers = append(marshalers, jsonv2.MarshalToFunc[any](func(encoder *jsontext.Encoder, value any) error {
		if err := contextstate.Validate(ctx); err != nil {
			return &nativeJSONError{cause: err}
		}
		observed := reflect.ValueOf(value)
		for observed.IsValid() {
			if observed.CanInterface() && nativeOwnedMarshaler(observed.Interface()) {
				return errors.ErrUnsupported
			}
			if observed.Kind() != reflect.Pointer {
				break
			}
			if observed.IsNil() {
				return errors.ErrUnsupported
			}
			observed = observed.Elem()
		}
		if !observed.IsValid() {
			return errors.ErrUnsupported
		}
		if (observed.Kind() == reflect.Struct || observed.Kind() == reflect.Array || observed.Kind() == reflect.Slice) && encoder.StackDepth() >= int(core.JSONNestingDepthMaximum) {
			return &DepthError{Maximum: core.JSONNestingDepthMaximum, Observed: encoder.StackDepth() + 1}
		}
		if observed.Kind() == reflect.Map {
			return core.ErrJSONContract
		}
		if observed.Kind() != reflect.Struct {
			return errors.ErrUnsupported
		}
		ordered, changed, err := orderDeclaredProjection(observed)
		if err != nil {
			return err
		}
		if !changed {
			return errors.ErrUnsupported
		}
		return jsonv2.MarshalEncode(encoder, ordered.Interface())
	}))
	writer := ownedCodecDestination{ctx: ctx, destination: destination}
	return objectReadOutcome(ctx, jsonv2.MarshalWrite(writer, document,
		jsonv2.Deterministic(true), jsonv2.WithMarshalers(jsonv2.JoinMarshalers(marshalers...)),
		jsontext.ReorderRawObjects(true), jsontext.CanonicalizeRawInts(false), jsontext.CanonicalizeRawFloats(true)))
}

func nativeOwnedMarshaler(value any) bool {
	if _, ok := value.(interface{ MarshalJSON() ([]byte, error) }); ok {
		return true
	}
	if _, ok := value.(interface{ MarshalJSONTo(*jsontext.Encoder) error }); ok {
		return true
	}
	if _, ok := value.(encoding.TextAppender); ok {
		return true
	}
	_, ok := value.(encoding.TextMarshaler)
	return ok
}

type canonicalDeclaredField struct {
	field reflect.StructField
	value reflect.Value
	name  string
}

func orderDeclaredProjection(value reflect.Value) (reflect.Value, bool, error) {
	fields, flattened, err := declaredProjectionFields(value, nil)
	if err != nil {
		return reflect.Value{}, false, err
	}
	ordered := slices.IsSortedFunc(fields, compareDeclaredFields)
	if !ordered {
		slices.SortFunc(fields, compareDeclaredFields)
	}
	for i := 1; i < len(fields); i++ {
		if fields[i-1].name == fields[i].name {
			return reflect.Value{}, false, core.ErrJSONContract
		}
	}
	if ordered && !flattened {
		return value, false, nil
	}
	declaration := make([]reflect.StructField, len(fields))
	for i, field := range fields {
		declaration[i] = reflect.StructField{Name: "Field" + strconv.Itoa(i), Type: field.field.Type, Tag: field.field.Tag}
	}
	projection := reflect.New(reflect.StructOf(declaration)).Elem()
	for i, field := range fields {
		projection.Field(i).Set(field.value)
	}
	return projection, true, nil
}

func declaredProjectionFields(value reflect.Value, fields []canonicalDeclaredField) ([]canonicalDeclaredField, bool, error) {
	flattened := false
	for i := 0; i < value.NumField(); i++ {
		field := value.Type().Field(i)
		if field.PkgPath != "" {
			continue
		}
		tag, tagged := field.Tag.Lookup("json")
		if tag == "-" {
			continue
		}
		if field.Anonymous && tag == "" && field.Type.Kind() == reflect.Struct {
			var err error
			fields, _, err = declaredProjectionFields(value.Field(i), fields)
			if err != nil {
				return nil, false, err
			}
			flattened = true
			continue
		}
		name, options, _ := strings.Cut(tag, ",")
		if !tagged || name == "" || strings.HasPrefix(name, "'") || strings.Contains(options, "inline") || strings.Contains(options, "unknown") {
			return nil, false, core.ErrJSONContract
		}
		fields = append(fields, canonicalDeclaredField{field: field, value: value.Field(i), name: name})
	}
	return fields, flattened, nil
}

func compareDeclaredFields(a, b canonicalDeclaredField) int {
	return slices.Compare(utf16.Encode([]rune(a.name)), utf16.Encode([]rune(b.name)))
}
