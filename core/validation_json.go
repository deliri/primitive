package core

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Validatable is implemented by values that own a boundary invariant.
type Validatable interface {
	Validate() error
}

// ValidatedJSONMarshaler is a value with an explicit JSON representation and
// an owner-enforced invariant. Requiring json.Marshaler prevents opaque typed
// values from silently encoding as empty objects.
type ValidatedJSONMarshaler interface {
	Validatable
	json.Marshaler
}

// OffWireEnum is the compiler-visible positive declaration that a validated
// closed enum is intentionally off wire. Go interfaces cannot express method
// absence, so each declaring package separately proves that its enum implements
// no marshaler. This interface owns the marker method instead of leaving its
// name as an informal convention repeated across packages.
type OffWireEnum interface {
	Validatable
	OffWireEnum()
}

const (
	// JSONDocumentMaximumBytes is the one-mebibyte document cap.
	JSONDocumentMaximumBytes = 1 << 20
	// JSONNestingDepthMaximum is the open-container cap.
	JSONNestingDepthMaximum = 64
	// JSONObjectFieldCountMaximum is the per-object field cap.
	JSONObjectFieldCountMaximum = 256
	// JSONArrayItemCountMaximum is the per-array item cap.
	JSONArrayItemCountMaximum             = 1024
	jsonObjectOverheadBytes               = len(`{}`)
	jsonArrayOverheadBytes                = len(`[]`)
	jsonNullLiteralText                   = "null"
	jsonDocumentByteLimitInvalidErrorText = "json document byte limit is invalid"
	jsonMarshalerPanicErrorText           = "validated json marshaler panicked"
	jsonValidatorPanicErrorText           = "validated json validator panicked"
	jsonUnmarshalerPanicErrorText         = "strict json unmarshaler panicked"
	jsonDocumentLimitExceededErrorText    = "json document exceeds byte limit"
	jsonDocumentInvalidUTF8ErrorText      = "json document is not valid utf-8"
	jsonObjectFieldLimitExceededErrorText = "json object exceeds field limit"
	jsonNestingLimitExceededErrorText     = "json nesting exceeds depth limit"
	jsonArrayItemLimitExceededErrorText   = "json array exceeds item limit"
	jsonRepresentationUnstableErrorText   = "validated json representation is not stable"
	jsonDecodedValueInvalidErrorText      = "decoded json value is invalid"
	jsonMismatchedDelimiterErrorText      = "json delimiter does not close current container"
)

// StrictJSONLimits supplies positive, caller-owned bounds for one JSON
// operation. The limits bound the input and Core's structural scan; they cannot
// bound work performed by T's UnmarshalJSON or Validate methods. Callers may
// choose limits at or below the package maxima.
type StrictJSONLimits struct {
	// DocumentMaximumBytes bounds the complete encoded document.
	DocumentMaximumBytes ByteCount
	// NestingDepthMaximum bounds simultaneously open arrays and objects.
	NestingDepthMaximum uint16
	// ObjectFieldMaximum bounds fields in each object.
	ObjectFieldMaximum uint16
	// ArrayItemMaximum bounds items in each array.
	ArrayItemMaximum uint32
}

// DefaultStrictJSONLimits returns the documented bounded JSON policy.
func DefaultStrictJSONLimits() StrictJSONLimits {
	return StrictJSONLimits{
		DocumentMaximumBytes: ByteCount{value: JSONDocumentMaximumBytes},
		NestingDepthMaximum:  JSONNestingDepthMaximum,
		ObjectFieldMaximum:   JSONObjectFieldCountMaximum,
		ArrayItemMaximum:     JSONArrayItemCountMaximum,
	}
}

// Validate rejects zero or globally unsupported JSON limits.
func (l StrictJSONLimits) Validate() error {
	if err := l.DocumentMaximumBytes.Validate(); err != nil {
		return jsonContractError(jsonDocumentByteLimitInvalidErrorText, err)
	}
	if l.DocumentMaximumBytes.value > JSONDocumentMaximumBytes {
		return jsonContractError("json document byte limit exceeds the supported maximum", nil)
	}
	if l.NestingDepthMaximum == 0 || l.NestingDepthMaximum > JSONNestingDepthMaximum {
		return jsonContractError("json nesting depth limit is outside the supported range", nil)
	}
	if l.ObjectFieldMaximum == 0 || l.ObjectFieldMaximum > JSONObjectFieldCountMaximum {
		return jsonContractError("json object field limit is outside the supported range", nil)
	}
	if l.ArrayItemMaximum == 0 || l.ArrayItemMaximum > JSONArrayItemCountMaximum {
		return jsonContractError("json array item limit is outside the supported range", nil)
	}
	return nil
}

// EncodeValidatedJSON validates limits and value, then emits one strict JSON
// document that DecodeStrictJSON can consume without semantic loss. The
// encoded representation must decode into T, including through methods on
// *T, pass T.Validate, and re-encode to the same bytes; the generic constraint
// cannot express that value/pointer symmetry, so this function enforces it.
// The package-wide case-insensitive object-key uniqueness rule applies to
// encoded output at every nesting level. Invalid UTF-8, unpaired JSON
// surrogate escapes, and any explicit null representation are rejected.
func EncodeValidatedJSON[T ValidatedJSONMarshaler](value T, limits StrictJSONLimits) ([]byte, error) {
	if err := limits.Validate(); err != nil {
		return nil, err
	}
	if err := validateJSONValue(value); err != nil {
		return nil, jsonContractError("validated json value is invalid", err)
	}
	encoded, err := marshalValidatedJSON(value)
	if err != nil {
		return nil, jsonContractError("validated json encoding failed", err)
	}
	if err := validateInitialJSONEncoding(encoded); err != nil {
		return nil, err
	}
	decoded, err := decodeStrictJSONStructureValidatedLimits[T](encoded, limits)
	if err != nil {
		return nil, jsonContractError("validated json encoding violates the strict wire contract", err)
	}
	if err := validateJSONValue(decoded); err != nil {
		return nil, jsonContractError("validated json decoded value is invalid", err)
	}
	reencoded, err := marshalValidatedJSON(decoded)
	if err != nil {
		return nil, jsonContractError("validated json decoded value cannot be reencoded", err)
	}
	if err := validateReencodedJSON(reencoded); err != nil {
		return nil, err
	}
	if !bytes.Equal(encoded, reencoded) {
		return nil, jsonContractError(jsonRepresentationUnstableErrorText, nil)
	}
	return encoded, nil
}

func validateJSONValue(value Validatable) (err error) {
	defer func() {
		if recover() != nil {
			err = errors.New(jsonValidatorPanicErrorText)
		}
	}()
	return value.Validate()
}

func validateInitialJSONEncoding(encoded []byte) error {
	if !json.Valid(encoded) {
		return jsonContractError("validated json marshaler emitted invalid json", nil)
	}
	if bytes.Equal(bytes.TrimSpace(encoded), []byte(jsonNullLiteralText)) {
		return jsonContractError("validated json representation is null", nil)
	}
	return nil
}

func validateReencodedJSON(encoded []byte) error {
	if !json.Valid(encoded) {
		return jsonContractError("validated json decoded value emitted invalid json", nil)
	}
	return nil
}

func marshalValidatedJSON(value json.Marshaler) (encoded []byte, err error) {
	defer func() {
		if recover() != nil {
			encoded = nil
			err = errors.New(jsonMarshalerPanicErrorText)
		}
	}()
	return value.MarshalJSON()
}

// DecodeStrictJSON decodes one strict JSON document into T. It rejects invalid
// UTF-8, unpaired JSON surrogate escapes, unknown fields, case-insensitive
// duplicate object keys at every nesting level, trailing data, null documents,
// and configured-limit violations. The uniqueness rule applies package-wide,
// not only to Go structs.
//
// Strict describes rejection and resource rules, not an injective or canonical
// byte representation. Leading and trailing whitespace and equivalent JSON
// string escapes may decode to the same typed value. A protocol that signs or
// hashes wire bytes must authenticate the original bytes before decoding; a
// protocol that signs typed values must define its own canonical projection.
// Every failure returns the zero value of T.
func DecodeStrictJSON[T Validatable](data []byte, limits StrictJSONLimits) (T, error) {
	var zero T
	if err := limits.Validate(); err != nil {
		return zero, err
	}
	if bytes.Equal(bytes.TrimSpace(data), []byte(jsonNullLiteralText)) {
		return zero, jsonContractError("decoded json document is null", nil)
	}
	value, err := decodeStrictJSONStructureValidatedLimits[T](data, limits)
	if err != nil {
		return zero, err
	}
	if err := validateJSONValue(value); err != nil {
		return zero, jsonContractError(jsonDecodedValueInvalidErrorText, err)
	}
	return value, nil
}

// DecodeStrictJSONStructure applies the complete strict JSON grammar and
// configured bounds without invoking T.Validate. It exists for typed boundary
// projection: a caller decodes into a private temporary, adds non-wire request
// state, validates the completed owning value, and only then permits it to
// escape. Callers that do not need that exact sequence use DecodeStrictJSON.
func DecodeStrictJSONStructure[T any](
	data []byte,
	limits StrictJSONLimits,
) (T, error) {
	if err := limits.Validate(); err != nil {
		var zero T
		return zero, err
	}
	return decodeStrictJSONStructureValidatedLimits[T](data, limits)
}

func decodeStrictJSONStructureValidatedLimits[T any](data []byte, limits StrictJSONLimits) (T, error) {
	var value T
	if err := validateStrictJSONTypedInput[T](data, limits); err != nil {
		return value, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decodeJSONValue(decoder, &value); err != nil {
		var zero T
		return zero, jsonContractError("strict json decode failed", err)
	}
	if err := rejectTrailingJSON(decoder); err != nil {
		var zero T
		return zero, err
	}
	return value, nil
}

func decodeJSONValue[T any](decoder *json.Decoder, destination *T) (err error) {
	defer func() {
		if recover() != nil {
			err = errors.New(jsonUnmarshalerPanicErrorText)
		}
	}()
	return decoder.Decode(destination)
}

func validateStrictJSONTypedInput[T any](data []byte, limits StrictJSONLimits) error {
	return validateStrictJSONInputWithFields(
		data,
		limits,
		jsonFieldNamesForType(reflect.TypeFor[T]()),
	)
}

func validateStrictJSONInputWithFields(
	data []byte,
	limits StrictJSONLimits,
	fields []string,
) error {
	if len(data) == 0 {
		return jsonContractError("json document is empty", nil)
	}
	if uint64(len(data)) > limits.DocumentMaximumBytes.value {
		return jsonContractError(jsonDocumentLimitExceededErrorText, nil)
	}
	if !utf8.Valid(data) {
		return jsonContractError(jsonDocumentInvalidUTF8ErrorText, nil)
	}
	if err := rejectUnpairedJSONSurrogates(data); err != nil {
		return err
	}
	return rejectDuplicateJSONFieldsWithFields(data, limits, fields)
}

func rejectTrailingJSON(decoder *json.Decoder) error {
	_, err := decoder.Token()
	if errors.Is(err, io.EOF) {
		return nil
	}
	return jsonContractError("json document has trailing data", err)
}

type strictJSONContainerKind uint8

const (
	strictJSONContainerObject strictJSONContainerKind = iota + 1
	strictJSONContainerArray
)

type strictJSONContainer struct {
	keys      []string
	itemCount uint32
	kind      strictJSONContainerKind
	expectKey bool
}

type jsonContractDiagnostic struct {
	message string
}

// Error returns the operator-facing strict JSON diagnostic.
func (d jsonContractDiagnostic) Error() string {
	return d.message
}

func rejectDuplicateJSONFieldsWithFields(
	data []byte,
	limits StrictJSONLimits,
	fields []string,
) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	stack := make([]strictJSONContainer, 0)
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			if len(stack) != 0 {
				return jsonContractError("json document has unclosed container", nil)
			}
			return nil
		}
		if err != nil {
			return jsonContractError("json token scan failed", err)
		}
		stack, err = scanStrictJSONToken(stack, token, limits, fields)
		if err != nil {
			return err
		}
	}
}

func scanStrictJSONToken(
	stack []strictJSONContainer,
	token json.Token,
	limits StrictJSONLimits,
	fields []string,
) ([]strictJSONContainer, error) {
	if delimiter, ok := token.(json.Delim); ok {
		return scanStrictJSONDelimiter(stack, delimiter, limits)
	}
	if key, ok := token.(string); ok && topExpectsObjectKey(stack) {
		return scanStrictJSONObjectKey(stack, key, limits, fields)
	}
	return completeStrictJSONValue(stack, limits)
}

func scanStrictJSONDelimiter(
	stack []strictJSONContainer,
	delimiter json.Delim,
	limits StrictJSONLimits,
) ([]strictJSONContainer, error) {
	switch delimiter {
	case '{':
		return pushStrictJSONContainer(stack, strictJSONContainerObject, limits)
	case '[':
		return pushStrictJSONContainer(stack, strictJSONContainerArray, limits)
	case '}', ']':
		if !delimiterClosesTop(stack, delimiter) {
			return nil, jsonContractError(jsonMismatchedDelimiterErrorText, nil)
		}
		if delimiter == '}' {
			if err := rejectDuplicateStrictJSONObjectKeys(stack[len(stack)-1].keys); err != nil {
				return nil, err
			}
		}
		return completeStrictJSONValue(stack[:len(stack)-1], limits)
	default:
		return nil, jsonContractError("json delimiter is invalid", nil)
	}
}

func pushStrictJSONContainer(
	stack []strictJSONContainer,
	kind strictJSONContainerKind,
	limits StrictJSONLimits,
) ([]strictJSONContainer, error) {
	if len(stack) >= int(limits.NestingDepthMaximum) {
		return nil, jsonContractError(jsonNestingLimitExceededErrorText, nil)
	}
	container := strictJSONContainer{kind: kind}
	if kind == strictJSONContainerObject {
		container.expectKey = true
	}
	return append(stack, container), nil
}

func delimiterClosesTop(stack []strictJSONContainer, delimiter json.Delim) bool {
	if len(stack) == 0 {
		return false
	}
	top := stack[len(stack)-1]
	if delimiter == '}' {
		return top.kind == strictJSONContainerObject && top.expectKey
	}
	return top.kind == strictJSONContainerArray
}

func topExpectsObjectKey(stack []strictJSONContainer) bool {
	if len(stack) == 0 {
		return false
	}
	top := stack[len(stack)-1]
	return top.kind == strictJSONContainerObject && top.expectKey
}

func scanStrictJSONObjectKey(
	stack []strictJSONContainer,
	key string,
	limits StrictJSONLimits,
	fields []string,
) ([]strictJSONContainer, error) {
	top := &stack[len(stack)-1]
	if len(top.keys) >= int(limits.ObjectFieldMaximum) {
		return nil, jsonContractError(jsonObjectFieldLimitExceededErrorText, nil)
	}
	if matchesDeclaredJSONFieldOnlyByCaseFold(fields, key) {
		return nil, jsonContractError("json object field casing is not canonical", nil)
	}
	top.keys = append(top.keys, foldStrictJSONKey(key))
	top.expectKey = false
	return stack, nil
}

func matchesDeclaredJSONFieldOnlyByCaseFold(fields []string, key string) bool {
	if _, exact := slices.BinarySearch(fields, key); exact {
		return false
	}
	return slices.ContainsFunc(fields, func(field string) bool {
		return strings.EqualFold(field, key)
	})
}

func jsonFieldNamesForType(root reflect.Type) []string {
	fields := make([]string, 0)
	pending := []reflect.Type{root}
	visited := make([]reflect.Type, 0)
	for len(pending) > 0 {
		last := len(pending) - 1
		valueType := indirectJSONFieldType(pending[last])
		pending = pending[:last]
		fields, pending, visited = collectJSONFieldType(
			valueType,
			fields,
			pending,
			visited,
		)
	}
	slices.Sort(fields)
	return slices.Compact(fields)
}

func collectJSONFieldType(
	valueType reflect.Type,
	fields []string,
	pending []reflect.Type,
	visited []reflect.Type,
) ([]string, []reflect.Type, []reflect.Type) {
	if valueType == nil || jsonFieldTypeOwnsContract(valueType) || valueType.Kind() == reflect.Interface {
		return fields, pending, visited
	}
	if valueType.Kind() == reflect.Map {
		return fields, append(pending, valueType.Elem()), visited
	}
	if valueType.Kind() != reflect.Struct || slices.Contains(visited, valueType) {
		return fields, pending, visited
	}
	visited = append(visited, valueType)
	for field := range valueType.Fields() {
		name, included, nested := reflectedJSONFieldName(field)
		if included {
			fields = append(fields, name)
		}
		if nested {
			pending = append(pending, field.Type)
		}
	}
	return fields, pending, visited
}

func indirectJSONFieldType(valueType reflect.Type) reflect.Type {
	for valueType != nil && (valueType.Kind() == reflect.Pointer || valueType.Kind() == reflect.Slice || valueType.Kind() == reflect.Array) {
		valueType = valueType.Elem()
	}
	return valueType
}

func jsonFieldTypeOwnsContract(valueType reflect.Type) bool {
	unmarshaler := reflect.TypeFor[json.Unmarshaler]()
	return valueType.Implements(unmarshaler) || reflect.PointerTo(valueType).Implements(unmarshaler)
}

func reflectedJSONFieldName(field reflect.StructField) (string, bool, bool) {
	if !field.IsExported() {
		return "", false, false
	}
	tag := field.Tag.Get("json")
	name, _, _ := strings.Cut(tag, ",")
	if name == "-" {
		return "", false, false
	}
	if name != "" {
		return name, true, true
	}
	fieldType := indirectJSONFieldType(field.Type)
	if field.Anonymous && fieldType != nil && fieldType.Kind() == reflect.Struct {
		return "", false, true
	}
	return field.Name, true, true
}

func rejectDuplicateStrictJSONObjectKeys(keys []string) error {
	slices.Sort(keys)
	for index := 1; index < len(keys); index++ {
		if keys[index] == keys[index-1] {
			return jsonContractError("json object contains duplicate field", nil)
		}
	}
	return nil
}

func foldStrictJSONKey(key string) string {
	if strictJSONKeyIsFoldedASCII(key) {
		return key
	}
	var folded strings.Builder
	folded.Grow(len(key))
	for _, value := range key {
		folded.WriteRune(canonicalSimpleFoldRune(value))
	}
	return folded.String()
}

func strictJSONKeyIsFoldedASCII(key string) bool {
	for index := 0; index < len(key); index++ {
		value := key[index]
		if value >= utf8.RuneSelf || value >= 'A' && value <= 'Z' {
			return false
		}
	}
	return true
}

func canonicalSimpleFoldRune(value rune) rune {
	canonical := unicode.ToLower(value)
	for candidate := unicode.SimpleFold(value); candidate != value; candidate = unicode.SimpleFold(candidate) {
		folded := unicode.ToLower(candidate)
		if folded < canonical {
			canonical = folded
		}
	}
	return canonical
}

func completeStrictJSONValue(
	stack []strictJSONContainer,
	limits StrictJSONLimits,
) ([]strictJSONContainer, error) {
	if len(stack) == 0 {
		return stack, nil
	}
	top := &stack[len(stack)-1]
	if top.kind == strictJSONContainerObject {
		if top.expectKey {
			return nil, jsonContractError("json object value is missing", nil)
		}
		top.expectKey = true
		return stack, nil
	}
	if top.itemCount >= limits.ArrayItemMaximum {
		return nil, jsonContractError(jsonArrayItemLimitExceededErrorText, nil)
	}
	top.itemCount++
	return stack, nil
}

func jsonContractError(message string, cause error) error {
	detail := jsonContractDiagnostic{message: message}
	if cause == nil {
		return errors.Join(ErrJSONContract, detail)
	}
	return errors.Join(ErrJSONContract, detail, cause)
}
