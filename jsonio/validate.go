package jsonio

import (
	"context"
	"encoding/json/jsontext"
	"errors"
	"io"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// ValidateDocument admits exactly one JSON value followed by clean EOF. Go owns
// grammar, duplicate-name and UTF-8 validation. SkipValue discards parsed values
// rather than retaining the complete document; Go's token and nesting working
// storage remains necessary. There is no total document byte ceiling. The caller
// owns the source's close and interruptible read lifetime.
func ValidateDocument(ctx context.Context, source io.Reader) error {
	if err := contextstate.Validate(ctx); err != nil {
		return err
	}
	if core.ReaderIsNil(source) {
		return core.ErrJSONContract
	}
	decoder := jsontext.NewDecoder(contextJSONSource{ctx: ctx, source: checkedJSONSource{source: source}})
	if err := decoder.SkipValue(); err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	if err := contextstate.Validate(ctx); err != nil {
		return err
	}
	err := decoder.SkipValue()
	if terminal := contextstate.Validate(ctx); terminal != nil {
		return errors.Join(terminal, err)
	}
	if err == io.EOF {
		return nil
	}
	return errors.Join(core.ErrJSONContract, err)
}

// contextJSONSource checks cancellation around every caller-owned read; it does
// not manufacture cancellation of a source that is already blocked in Read.
type contextJSONSource struct {
	ctx    context.Context
	source checkedJSONSource
}

func (r contextJSONSource) Read(data []byte) (int, error) {
	if err := contextstate.Validate(r.ctx); err != nil {
		return 0, err
	}
	n, err := r.source.Read(data)
	if terminal := contextstate.Validate(r.ctx); terminal != nil {
		return n, errors.Join(err, terminal)
	}
	return n, err
}
