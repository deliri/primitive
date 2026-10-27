package plunk

import (
	"bytes"
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"net/mail"

	"github.com/deliri/primitive/v2026/core"
	primitiveid "github.com/deliri/primitive/v2026/id"
	"github.com/deliri/primitive/v2026/jsonio"
)

// sendResponsePathMaximumTokens is the deepest known receipt path:
// data/emails/<index>/contact/<id|email>.
const sendResponsePathMaximumTokens = 5

// SendResponseField names the known receipt location of a shape refusal.
// Any other JSON pointer is Unknown; pointer text is never retained.
type SendResponseField uint8

const (
	SendResponseFieldUnknown SendResponseField = iota
	SendResponseFieldData
	SendResponseFieldEmails
	SendResponseFieldEmailItem
	SendResponseFieldEmailItemMessage
	SendResponseFieldEmailItemContact
	SendResponseFieldEmailItemContactID
	SendResponseFieldEmailItemContactEmail
	SendResponseFieldTimestamp
	SendResponseFieldEmailItemStatus
	SendResponseFieldEmailItemScheduledFor
	SendResponseFieldEmailItemNextAllowedAt
	sendResponseFieldLimit
)

func (f SendResponseField) Validate() error {
	if f >= sendResponseFieldLimit {
		return core.ErrPlunkContract
	}
	return nil
}

func (f SendResponseField) String() string {
	switch f {
	case SendResponseFieldUnknown:
		return "unknown"
	case SendResponseFieldData:
		return "data"
	case SendResponseFieldEmails:
		return "data.emails"
	case SendResponseFieldEmailItem:
		return "data.emails[]"
	case SendResponseFieldEmailItemMessage:
		return "data.emails[].email"
	case SendResponseFieldEmailItemContact:
		return "data.emails[].contact"
	case SendResponseFieldEmailItemContactID:
		return "data.emails[].contact.id"
	case SendResponseFieldEmailItemContactEmail:
		return "data.emails[].contact.email"
	case SendResponseFieldTimestamp:
		return "data.timestamp"
	case SendResponseFieldEmailItemStatus:
		return "data.emails[].status"
	case SendResponseFieldEmailItemScheduledFor:
		return "data.emails[].scheduledFor"
	case SendResponseFieldEmailItemNextAllowedAt:
		return "data.emails[].nextAllowedAt"
	default:
		return "invalid"
	}
}

// ClassifySendResponseField maps a native JSON pointer onto the known receipt
// paths. Every array index collapses to EmailItem; anything else is Unknown.
func ClassifySendResponseField(pointer jsontext.Pointer) SendResponseField {
	if !pointer.IsValid() {
		return SendResponseFieldUnknown
	}
	var path [sendResponsePathMaximumTokens]string
	count := 0
	for token := range pointer.Tokens() {
		if count == len(path) {
			return SendResponseFieldUnknown
		}
		path[count] = token
		count++
	}
	switch {
	case count == 0 || path[0] != "data":
		return SendResponseFieldUnknown
	case count == 1:
		return SendResponseFieldData
	case count == 2 && path[1] == "emails":
		return SendResponseFieldEmails
	case count == 2 && path[1] == "timestamp":
		return SendResponseFieldTimestamp
	case count == 2 || path[1] != "emails" || !sendResponseIndex(path[2]):
		return SendResponseFieldUnknown
	case count == 3:
		return SendResponseFieldEmailItem
	case count == 4 && path[3] == "email":
		return SendResponseFieldEmailItemMessage
	case count == 4 && path[3] == "status":
		return SendResponseFieldEmailItemStatus
	case count == 4 && path[3] == "scheduledFor":
		return SendResponseFieldEmailItemScheduledFor
	case count == 4 && path[3] == "nextAllowedAt":
		return SendResponseFieldEmailItemNextAllowedAt
	case path[3] != "contact":
		return SendResponseFieldUnknown
	case count == 4:
		return SendResponseFieldEmailItemContact
	case path[4] == "id":
		return SendResponseFieldEmailItemContactID
	case path[4] == "email":
		return SendResponseFieldEmailItemContactEmail
	default:
		return SendResponseFieldUnknown
	}
}

func sendResponseIndex(token string) bool {
	if token == "" {
		return false
	}
	for _, digit := range []byte(token) {
		if digit < '0' || digit > '9' {
			return false
		}
	}
	return true
}

// SendResponseJSONKind is the native JSON kind found where the receipt DTO
// expected another shape.
type SendResponseJSONKind uint8

const (
	SendResponseJSONKindUnknown SendResponseJSONKind = iota
	SendResponseJSONKindNull
	SendResponseJSONKindFalse
	SendResponseJSONKindTrue
	SendResponseJSONKindString
	SendResponseJSONKindNumber
	SendResponseJSONKindObject
	SendResponseJSONKindArray
	sendResponseJSONKindLimit
)

func (k SendResponseJSONKind) Validate() error {
	if k >= sendResponseJSONKindLimit {
		return core.ErrPlunkContract
	}
	return nil
}

func (k SendResponseJSONKind) String() string {
	switch k {
	case SendResponseJSONKindUnknown:
		return "unknown"
	case SendResponseJSONKindNull:
		return "null"
	case SendResponseJSONKindFalse:
		return "false"
	case SendResponseJSONKindTrue:
		return "true"
	case SendResponseJSONKindString:
		return "string"
	case SendResponseJSONKindNumber:
		return "number"
	case SendResponseJSONKindObject:
		return "object"
	case SendResponseJSONKindArray:
		return "array"
	default:
		return "invalid"
	}
}

func ClassifySendResponseJSONKind(kind jsontext.Kind) SendResponseJSONKind {
	switch kind {
	case jsontext.KindNull:
		return SendResponseJSONKindNull
	case jsontext.KindFalse:
		return SendResponseJSONKindFalse
	case jsontext.KindTrue:
		return SendResponseJSONKindTrue
	case jsontext.KindString:
		return SendResponseJSONKindString
	case jsontext.KindNumber:
		return SendResponseJSONKindNumber
	case jsontext.KindBeginObject:
		return SendResponseJSONKindObject
	case jsontext.KindBeginArray:
		return SendResponseJSONKindArray
	default:
		return SendResponseJSONKindUnknown
	}
}

// SendResponseStringShape classifies a string found at a known receipt field
// with existing validators only. It does not say what the provider meant.
type SendResponseStringShape uint8

const (
	SendResponseStringShapeNotApplicable SendResponseStringShape = iota
	SendResponseStringShapeUnavailable
	SendResponseStringShapeEmpty
	SendResponseStringShapeUUIDv7
	SendResponseStringShapeEmailAddress
	SendResponseStringShapeOpaque
	sendResponseStringShapeLimit
)

func (s SendResponseStringShape) Validate() error {
	if s >= sendResponseStringShapeLimit {
		return core.ErrPlunkContract
	}
	return nil
}

func (s SendResponseStringShape) String() string {
	switch s {
	case SendResponseStringShapeNotApplicable:
		return "not-applicable"
	case SendResponseStringShapeUnavailable:
		return "unavailable"
	case SendResponseStringShapeEmpty:
		return "empty"
	case SendResponseStringShapeUUIDv7:
		return "uuidv7"
	case SendResponseStringShapeEmailAddress:
		return "email-address"
	case SendResponseStringShapeOpaque:
		return "opaque"
	default:
		return "invalid"
	}
}

func ClassifySendResponseStringShape(value string) SendResponseStringShape {
	if value == "" {
		return SendResponseStringShapeEmpty
	}
	if _, err := primitiveid.ParseUUIDv7(value); err == nil {
		return SendResponseStringShapeUUIDv7
	}
	if address, err := mail.ParseAddress(value); err == nil && address.Address == value {
		return SendResponseStringShapeEmailAddress
	}
	return SendResponseStringShapeOpaque
}

// SendResponseShapeError retains only closed diagnostics in its text. Native
// parser causes remain reachable with errors.Is/As, never rendered or copied.
type SendResponseShapeError struct {
	cause error
	Field SendResponseField
	Kind  SendResponseJSONKind
	Shape SendResponseStringShape
}

func newSendResponseShapeError(ctx context.Context, document []byte, semantic *json.SemanticError, cause error) error {
	shape := &SendResponseShapeError{cause: cause}
	shape.Field = ClassifySendResponseField(semantic.JSONPointer)
	shape.Kind = ClassifySendResponseJSONKind(semantic.JSONKind)
	if shape.Field != SendResponseFieldUnknown && shape.Kind == SendResponseJSONKindString {
		shape.Shape = sendResponseStringShapeAt(ctx, document, semantic.ByteOffset)
	}
	return shape
}

func sendResponseStringShapeAt(ctx context.Context, document []byte, offset int64) SendResponseStringShape {
	if offset < 0 || offset >= int64(len(document)) {
		return SendResponseStringShapeUnavailable
	}
	cursor, err := jsonio.NewTokenDecoder(jsonio.TokenRequest{Source: bytes.NewReader(document[offset:]), NestingDepthMaximum: 1})
	if err != nil {
		return SendResponseStringShapeUnavailable
	}
	token, err := cursor.Decode(ctx)
	if err != nil || token.Kind != jsonio.TokenString {
		return SendResponseStringShapeUnavailable
	}
	return ClassifySendResponseStringShape(token.Text)
}

func (e *SendResponseShapeError) Error() string {
	return fmt.Sprintf("plunk: send response shape rejected: field=%s kind=%s shape=%s", e.Field, e.Kind, e.Shape)
}

func (e *SendResponseShapeError) Unwrap() error { return e.cause }
