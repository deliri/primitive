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
	borrowed := &contextJSONSource{ctx: ctx, source: checkedJSONSource{source: source}}
	decoder := jsontext.NewDecoder(borrowed)
	if err := borrowed.observedError(objectReadOutcome(ctx, decoder.SkipValue())); err != nil {
		if err == io.EOF {
			return core.ErrJSONContract
		}
		return err
	}
	if err := contextstate.Validate(ctx); err != nil {
		return err
	}
	err := borrowed.observedError(objectReadOutcome(ctx, decoder.SkipValue()))
	if terminal := contextstate.Validate(ctx); terminal != nil {
		return errors.Join(terminal, err)
	}
	if err == io.EOF {
		return nil
	}
	if err == nil {
		return core.ErrJSONContract
	}
	return err
}

// contextJSONSource checks cancellation around every caller-owned read; it does
// not manufacture cancellation of a source that is already blocked in Read.
type contextJSONSource struct {
	ctx     context.Context
	source  checkedJSONSource
	refusal error
}

func (r *contextJSONSource) Read(data []byte) (int, error) {
	if err := contextstate.Validate(r.ctx); err != nil {
		return 0, err
	}
	n, err := r.source.Read(data)
	if err != nil && err != io.EOF && r.refusal == nil {
		r.refusal = err
	}
	if terminal := contextstate.Validate(r.ctx); terminal != nil {
		return n, errors.Join(err, terminal)
	}
	return n, err
}

// A native decoder may read ahead, observe a source refusal and prefer a
// buffered grammar error. Keep that first source cause beside the result.
// Only a bare EOF is framing. Reset replaces the whole borrowed source.
func (r *contextJSONSource) observedError(err error) error {
	if r.refusal == nil {
		return err
	}
	if err == io.EOF {
		return r.refusal
	}
	return errors.Join(err, r.refusal)
}
