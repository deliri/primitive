package keygen

import (
	"bufio"
	"context"
	"errors"
	"io"
	"unicode"

	"github.com/deliri/primitive/v2026/core"
)

type PublicKeyHexReadRequest struct{ Source io.Reader }

func (r PublicKeyHexReadRequest) Validate() error {
	if core.ReaderIsNil(r.Source) {
		return core.ErrKeygenContract
	}
	return nil
}

// ReadPublicKeyHex admits Core's canonical Ed25519 hex with surrounding Unicode
// whitespace. Only the fixed 64-byte key token is retained. A native source
// refusal, including one after the complete token, withdraws the result.
func ReadPublicKeyHex(ctx context.Context, request PublicKeyHexReadRequest) (core.Ed25519PublicKey, error) {
	if ctx == nil {
		return core.Ed25519PublicKey{}, errors.Join(core.ErrKeygenContract, core.ErrNilContext)
	}
	if err := errors.Join(ctx.Err(), context.Cause(ctx), request.Validate()); err != nil {
		return core.Ed25519PublicKey{}, contractError(err)
	}
	source := bufio.NewReaderSize(keyStream{ctx: ctx, source: request.Source}, 4096)
	var text [2 * openSSHPublicBytes]byte
	count := 0
	trailing := false
	for {
		value, width, err := source.ReadRune()
		if err == io.EOF {
			break
		}
		if err != nil {
			return core.Ed25519PublicKey{}, contractError(err)
		}
		if unicode.IsSpace(value) {
			if count > 0 {
				trailing = true
			}
			continue
		}
		if trailing || width != 1 || value > 127 || count == len(text) {
			return core.Ed25519PublicKey{}, core.ErrKeygenPublicKeyText
		}
		text[count] = byte(value)
		count++
	}
	if err := ctx.Err(); err != nil {
		return core.Ed25519PublicKey{}, contractError(errors.Join(err, context.Cause(ctx)))
	}
	var result core.Ed25519PublicKey
	if err := result.UnmarshalText(text[:count]); err != nil {
		return core.Ed25519PublicKey{}, errors.Join(core.ErrKeygenPublicKeyText, err)
	}
	return result, nil
}
