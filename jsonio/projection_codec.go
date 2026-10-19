package jsonio

import (
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"io"
	"reflect"

	"github.com/deliri/primitive/v2026/core"
)

// MarshalProjection encodes one concrete wire projection through Go. It does
// not validate product meaning. RootDeclaredFields admits hook-free structs;
// RootOwnedMethods explicitly lends nested/root codecs to their owners. Output
// allocation follows the requested value, without a document-size quota.
func MarshalProjection[Document any](document Document, policy OwnedCodecPolicy) ([]byte, error) {
	if err := validateOwnedCodecType[Document](policy); err != nil {
		return nil, err
	}
	value := reflect.ValueOf(document)
	if value.Kind() == reflect.Pointer && value.IsNil() {
		return nil, core.ErrJSONContract
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

// DecodeProjection decodes a hook-free concrete struct with Go's structural
// admission. The owner validates meaning before committing its receiver. Root
// methods are deliberately refused so alias codecs cannot recursively call
// their owning public door. Native syntax causes survive; failure returns zero.
func DecodeProjection[Document any](data []byte, members MemberPolicy) (Document, error) {
	var zero Document
	if err := members.Validate(); err != nil {
		return zero, err
	}
	if err := validateObjectType[Document](); err != nil {
		return zero, err
	}
	value := jsontext.Value(data)
	if value.Kind() != jsontext.KindBeginObject {
		return zero, core.ErrJSONContract
	}
	// Validate through Go before admitting opaque nested field codecs.
	if err := checkSingleValue(data); err != nil {
		return zero, err
	}
	if err := checkCanonicalDepth(data, CanonicalPolicy{NestingDepthMaximum: core.JSONNestingDepthMaximum}); err != nil {
		return zero, err
	}
	var document Document
	if err := jsonv2.Unmarshal(data, &document, jsonv2.RejectUnknownMembers(members == MembersDeclaredOnly)); err != nil {
		return zero, jsonOperationError(err)
	}
	return document, nil
}

func checkSingleValue(data []byte) error {
	decoder := borrowedBytesDecoder(data)
	if err := decoder.SkipValue(); err != nil {
		return jsonOperationError(err)
	}
	_, err := decoder.ReadToken()
	if err == io.EOF {
		return nil
	}
	if err != nil {
		return jsonOperationError(err)
	}
	return &MultipleValuesError{}
}
