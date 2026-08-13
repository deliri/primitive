package core

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
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

// ValidatedJSONProjection is an intentionally one-way JSON value whose owner
// can prove that its exact emitted bytes satisfy the strict wire contract.
//
// EncodeValidatedJSON normally proves stability by decoding back into the
// producer type. That is deliberately impossible for issue-only bearer
// projections: only their distinct receive-only document may admit external
// bytes. This contract keeps those directions separate without weakening the
// encoder's strict grammar, resource, or exact-projection proof.
type ValidatedJSONProjection interface {
	ValidatedJSONMarshaler
	ValidateJSONProjection([]byte, StrictJSONLimits) error
}

// ValidateReceiveOnlyJSONProjection proves that exact issue-only bytes decode
// through their distinct receive-only owner and preserve the producer's
// canonical projection. Projection and document stay separate compiler-owned
// directions; neither needs a compatibility decoder or encoder.
func ValidateReceiveOnlyJSONProjection[
	Projection ValidatedJSONMarshaler,
	Document any,
	DocumentPtr interface {
		*Document
		Validatable
		json.Unmarshaler
	},
](projection Projection, encoded []byte, limits StrictJSONLimits) error {
	if err := limits.Validate(); err != nil {
		return err
	}
	if err := validateJSONValue(projection); err != nil {
		return jsonContractError("validated json projection is invalid", err)
	}
	decoded, err := DecodeStrictJSONStructure[Document](encoded, limits)
	if err != nil {
		return err
	}
	if err := validateJSONValue(DocumentPtr(&decoded)); err != nil {
		return jsonContractError("receive-only json document is invalid", err)
	}
	canonical, err := marshalValidatedJSON(projection)
	if err != nil {
		return jsonContractError("validated json projection cannot be reencoded", err)
	}
	if err := validateInitialJSONEncoding(canonical); err != nil {
		return err
	}
	if !bytes.Equal(encoded, canonical) {
		return jsonContractError(jsonRepresentationUnstableErrorText, nil)
	}
	return nil
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
	// JSONDocumentMaximumBytes is the shared one-mebibyte document cap.
	JSONDocumentMaximumBytes = 1 << 20
	// CredentialedRequestDocumentSyntaxBytes is the exact outer punctuation
	// shared by request-plus-installation-certificate documents.
	CredentialedRequestDocumentSyntaxBytes = len(`{"request":,"certificate":}`)
	// CredentialedDocumentWhitespaceMaximumBytes is the shared allowance for
	// insignificant outer whitespace around one credentialed document.
	CredentialedDocumentWhitespaceMaximumBytes = 8 << 10
	// JSONNestingDepthMaximum is the open-container cap.
	JSONNestingDepthMaximum = 64
	// JSONObjectFieldCountMaximum is the per-object field cap.
	JSONObjectFieldCountMaximum = 256
	// jsonArrayItemCountMaximum is the per-array item cap.
	jsonArrayItemCountMaximum             = 1024
	jsonObjectOverheadBytes               = len(`{}`)
	jsonArrayOverheadBytes                = len(`[]`)
	jsonNullLiteralText                   = "null"
	jsonDocumentByteLimitInvalidErrorText = "json document byte limit is invalid"
	jsonMarshalerPanicErrorText           = "validated json marshaler panicked"
	jsonValidatorPanicErrorText           = "validated json validator panicked"
	jsonProjectionValidatorPanicErrorText = "validated json projection validator panicked"
	jsonUnmarshalerPanicErrorText         = "strict json unmarshaler panicked"
	jsonDocumentLimitExceededErrorText    = "json document exceeds byte limit"
	jsonDocumentInvalidUTF8ErrorText      = "json document is not valid utf-8"
	jsonObjectFieldLimitExceededErrorText = "json object exceeds field limit"
	jsonNestingLimitExceededErrorText     = "json nesting exceeds depth limit"
	jsonArrayItemLimitExceededErrorText   = "json array exceeds item limit"
	jsonRepresentationUnstableErrorText   = "validated json representation is not stable"
	jsonDecodedValueInvalidErrorText      = "decoded json value is invalid"
	jsonMismatchedDelimiterErrorText      = "json delimiter does not close current container"
	jsonContainerKindInvalidErrorText     = "json container kind is invalid"
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
		ArrayItemMaximum:     jsonArrayItemCountMaximum,
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
	if l.ArrayItemMaximum == 0 || l.ArrayItemMaximum > jsonArrayItemCountMaximum {
		return jsonContractError("json array item limit is outside the supported range", nil)
	}
	return nil
}

// EncodeValidatedJSON validates limits and value, then emits one strict JSON
// document. A bidirectional value must decode into T, including through
// methods on *T, pass T.Validate, and re-encode to the same bytes; the generic
// constraint cannot express that value/pointer symmetry, so this function
// enforces it. An issue-only ValidatedJSONProjection instead proves its exact
// emitted bytes through its distinct compiler-owned projection validator.
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
	if projection, ok := any(value).(ValidatedJSONProjection); ok {
		if err := validateJSONProjection(projection, encoded, limits); err != nil {
			return nil, jsonContractError("validated json projection violates the strict wire contract", err)
		}
		return encoded, nil
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

func validateJSONProjection(
	projection ValidatedJSONProjection,
	encoded []byte,
	limits StrictJSONLimits,
) (err error) {
	defer func() {
		if recover() != nil {
			err = errors.New(jsonProjectionValidatorPanicErrorText)
		}
	}()
	return projection.ValidateJSONProjection(encoded, limits)
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
		cachedJSONFieldNamesForType(reflect.TypeFor[T]()),
	)
}

const (
	jsonFieldNameCacheEntryMaximum         = 128
	jsonFieldNameCacheFieldsPerTypeMaximum = 1024
)

// jsonFieldNameCache is a bounded, derived optimization over immutable Go type
// metadata. It is not protocol state: when either bound is reached, decoding
// remains correct and simply performs the reflective walk for that call.
type jsonFieldNameCache struct {
	values  sync.Map
	entries atomic.Uint32
}

var strictJSONFieldNames jsonFieldNameCache

func cachedJSONFieldNamesForType(root reflect.Type) []string {
	return strictJSONFieldNames.lookup(root)
}

func (c *jsonFieldNameCache) lookup(root reflect.Type) []string {
	if cached, ok := c.values.Load(root); ok {
		return cached.([]string)
	}
	fields := jsonFieldNamesForType(root)
	if len(fields) > jsonFieldNameCacheFieldsPerTypeMaximum || !c.reserve() {
		return fields
	}
	canonical, loaded := c.values.LoadOrStore(root, fields)
	if loaded {
		c.entries.Add(^uint32(0))
	}
	return canonical.([]string)
}

func (c *jsonFieldNameCache) reserve() bool {
	for {
		entries := c.entries.Load()
		if entries >= jsonFieldNameCacheEntryMaximum {
			return false
		}
		if c.entries.CompareAndSwap(entries, entries+1) {
			return true
		}
	}
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
	strictJSONContainerUnknown strictJSONContainerKind = iota
	strictJSONContainerObject
	strictJSONContainerArray
	strictJSONContainerKindLimit
)

func strictJSONContainerKindLabels() [strictJSONContainerKindLimit]string {
	return [strictJSONContainerKindLimit]string{
		strictJSONContainerObject: "object",
		strictJSONContainerArray:  "array",
	}
}

// IsValid reports whether the internal parser discriminator is admitted.
func (k strictJSONContainerKind) IsValid() bool {
	labels := strictJSONContainerKindLabels()
	return k > strictJSONContainerUnknown &&
		k < strictJSONContainerKindLimit &&
		labels[k] != ""
}

// Validate rejects an unset or future parser discriminator.
func (k strictJSONContainerKind) Validate() error {
	if !k.IsValid() {
		return jsonContractError(jsonContainerKindInvalidErrorText, nil)
	}
	return nil
}

// OffWireEnum declares that the parser discriminator has no wire encoding.
func (strictJSONContainerKind) OffWireEnum() {}

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
		stack, err = scanStrictJSONToken(strictJSONTokenScan{
			stack: stack, token: token, limits: limits, fields: fields,
		})
		if err != nil {
			return err
		}
	}
}

type strictJSONTokenScan struct {
	stack  []strictJSONContainer
	token  json.Token
	limits StrictJSONLimits
	fields []string
}

func scanStrictJSONToken(scan strictJSONTokenScan) ([]strictJSONContainer, error) {
	if delimiter, ok := scan.token.(json.Delim); ok {
		return scanStrictJSONDelimiter(scan.stack, delimiter, scan.limits)
	}
	if key, ok := scan.token.(string); ok && topExpectsObjectKey(scan.stack) {
		return scanStrictJSONObjectKey(strictJSONObjectKeyScan{
			stack: scan.stack, key: key, limits: scan.limits, fields: scan.fields,
		})
	}
	return completeStrictJSONValue(scan.stack, scan.limits)
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
	if err := kind.Validate(); err != nil {
		return nil, err
	}
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

type strictJSONObjectKeyScan struct {
	stack  []strictJSONContainer
	key    string
	limits StrictJSONLimits
	fields []string
}

func scanStrictJSONObjectKey(scan strictJSONObjectKeyScan) ([]strictJSONContainer, error) {
	top := &scan.stack[len(scan.stack)-1]
	if len(top.keys) >= int(scan.limits.ObjectFieldMaximum) {
		return nil, jsonContractError(jsonObjectFieldLimitExceededErrorText, nil)
	}
	if matchesDeclaredJSONFieldOnlyByCaseFold(scan.fields, scan.key) {
		return nil, jsonContractError("json object field casing is not canonical", nil)
	}
	top.keys = append(top.keys, foldStrictJSONKey(scan.key))
	top.expectKey = false
	return scan.stack, nil
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
		fields, pending, visited = collectJSONFieldType(jsonFieldTypeCollection{
			valueType: valueType, fields: fields, pending: pending, visited: visited,
		})
	}
	slices.Sort(fields)
	return slices.Compact(fields)
}

type jsonFieldTypeCollection struct {
	valueType reflect.Type
	fields    []string
	pending   []reflect.Type
	visited   []reflect.Type
}

func collectJSONFieldType(collection jsonFieldTypeCollection) ([]string, []reflect.Type, []reflect.Type) {
	valueType := collection.valueType
	if valueType == nil || jsonFieldTypeOwnsContract(valueType) || valueType.Kind() == reflect.Interface {
		return collection.fields, collection.pending, collection.visited
	}
	if valueType.Kind() == reflect.Map {
		return collection.fields, append(collection.pending, valueType.Elem()), collection.visited
	}
	if valueType.Kind() != reflect.Struct || slices.Contains(collection.visited, valueType) {
		return collection.fields, collection.pending, collection.visited
	}
	collection.visited = append(collection.visited, valueType)
	for field := range valueType.Fields() {
		name, included, nested := reflectedJSONFieldName(field)
		if included {
			collection.fields = append(collection.fields, name)
		}
		if nested {
			collection.pending = append(collection.pending, field.Type)
		}
	}
	return collection.fields, collection.pending, collection.visited
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
	switch top.kind {
	case strictJSONContainerObject:
		if top.expectKey {
			return nil, jsonContractError("json object value is missing", nil)
		}
		top.expectKey = true
		return stack, nil
	case strictJSONContainerArray:
		if top.itemCount >= limits.ArrayItemMaximum {
			return nil, jsonContractError(jsonArrayItemLimitExceededErrorText, nil)
		}
		top.itemCount++
		return stack, nil
	default:
		return nil, jsonContractError(jsonContainerKindInvalidErrorText, nil)
	}
}

var (
	_ Validatable = strictJSONContainerUnknown
	_ OffWireEnum = strictJSONContainerUnknown
)

func jsonContractError(message string, cause error) error {
	detail := jsonContractDiagnostic{message: message}
	if cause == nil {
		return errors.Join(ErrJSONContract, detail)
	}
	return errors.Join(ErrJSONContract, detail, cause)
}
