package jsonio

import (
	"context"
	"encoding/json/jsontext"
	"errors"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// EncodeCanonicalObject emits one validated canonical object at the cursor's
// current value position. This explicitly retains that single object's bytes;
// it never collects its surrounding array or object. Callers requiring bounded
// sequence memory use scalar-record documents with no collection fields.
func EncodeCanonicalObject[Document core.Validatable](ctx context.Context, encoder *TokenEncoder, document Document, policy OwnedCodecPolicy) error {
	if err := errors.Join(contextstate.Validate(ctx), encoder.Validate()); err != nil {
		return err
	}
	if err := validateOwnedEncoding(document, policy); err != nil {
		return err
	}
	data, err := AppendCanonicalProjection(nil, document, policy, CanonicalPolicy{NestingDepthMaximum: core.JSONNestingDepthMaximum})
	if err != nil {
		return err
	}
	value := jsontext.Value(data)
	if value.Kind() != jsontext.KindBeginObject {
		return core.ErrJSONContract
	}
	if err := contextstate.Validate(ctx); err != nil {
		return err
	}
	return objectReadOutcome(ctx, encoder.encoder.WriteValue(value))
}
