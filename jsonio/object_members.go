package jsonio

import (
	"context"
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"errors"
	"io"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// ObjectMemberRequest lends a synchronous member producer to Go's object
// encoder. The callback writes names and values, without outer delimiters.
// It must not retain the borrowed token cursor.
type ObjectMemberRequest struct {
	Destination io.Writer
	Members     func(context.Context, *TokenEncoder) error
}

func (r ObjectMemberRequest) Validate() error {
	if err := (ObjectDestinationRequest{Destination: r.Destination}).Validate(); err != nil {
		return err
	}
	if r.Members == nil {
		return core.ErrJSONContract
	}
	return nil
}

// WriteObjectMembers writes one object without a trailing LF. Go owns grammar,
// duplicate-name checking, buffering and framing; callers own field policy.
// Retained memory follows open containers and the largest scalar. A failed
// member or destination can leave provisional bytes, which its owner discards.
func WriteObjectMembers(ctx context.Context, request ObjectMemberRequest) error {
	if err := errors.Join(contextstate.Validate(ctx), request.Validate()); err != nil {
		return err
	}
	err := jsonv2.MarshalWrite(ownedCodecDestination{ctx: ctx, destination: request.Destination}, memberObject{ctx: ctx, members: request.Members})
	return objectReadOutcome(ctx, err)
}

// This hook lends Go's existing cursor rather than nesting an encoder or
// collecting the member stream into a second document.
type memberObject struct {
	ctx     context.Context
	members func(context.Context, *TokenEncoder) error
}

func (o memberObject) MarshalJSONTo(encoder *jsontext.Encoder) error {
	depth := encoder.StackDepth()
	if err := encoder.WriteToken(jsontext.BeginObject); err != nil {
		return err
	}
	cursor := TokenEncoder{encoder: encoder}
	defer func() { cursor.encoder = nil }()
	if err := o.members(o.ctx, &cursor); err != nil {
		return err
	}
	if encoder.StackDepth() != depth+1 {
		return core.ErrJSONContract
	}
	if err := contextstate.Validate(o.ctx); err != nil {
		return err
	}
	return encoder.WriteToken(jsontext.EndObject)
}

var _ core.Validatable = ObjectMemberRequest{}
