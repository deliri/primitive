package jsonio

import (
	"context"
	"encoding/json/jsontext"
	"errors"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// EncodeCanonicalProjection emits one concrete value at the native cursor's
// current position. The caller owns meaning; Go owns its canonical bytes and
// the surrounding grammar. Refused projections do not advance the cursor.
// This operation retains that value's bytes. To stream a collection, emit its
// delimiters and call this operation once per scalar or individual record;
// passing the collection itself retains the complete collection.
func EncodeCanonicalProjection[Document any](ctx context.Context, encoder *TokenEncoder, document Document, policy OwnedCodecPolicy) error {
	if err := errors.Join(contextstate.Validate(ctx), encoder.Validate()); err != nil {
		return err
	}
	data, err := AppendCanonicalProjection(nil, document, policy, CanonicalPolicy{NestingDepthMaximum: core.JSONNestingDepthMaximum})
	if err != nil {
		return err
	}
	if err := contextstate.Validate(ctx); err != nil {
		return err
	}
	return objectReadOutcome(ctx, encoder.encoder.WriteValue(jsontext.Value(data)))
}
