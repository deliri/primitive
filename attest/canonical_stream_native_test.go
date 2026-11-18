package attest

import (
	"context"
	"crypto/sha256"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestCanonicalDigestWriterNativeFailureLayerTriad(t *testing.T) {
	t.Parallel()
	t.Run("native digest conserves bytes and exact extent", func(t *testing.T) {
		t.Parallel()
		writer := newCanonicalDigestWriter()
		data := []byte("native canonical stream")
		if n, err := writer.Write(data); err != nil || n != len(data) {
			t.Fatalf("Write = %d/%v, want %d/nil", n, err, len(data))
		}
		length, digest, err := writer.close(nil)
		count, countErr := length.Uint64()
		if err != nil || countErr != nil || count != uint64(len(data)) || digest != core.NewSHA256Digest(sha256.Sum256(data)) {
			t.Fatalf("close = %d/%v/%v/%v, want exact native digest and extent", count, digest, err, countErr)
		}
	})
	t.Run("native writer refusal latches and preserves callback cancellation", func(t *testing.T) {
		t.Parallel()
		writer := newCanonicalDigestWriter()
		if _, _, err := writer.digest.Seal(); err != nil {
			t.Fatal(err)
		}
		n, nativeErr := writer.Write([]byte{1})
		if n != 0 || !errors.Is(nativeErr, core.ErrAttestContract) || !errors.Is(nativeErr, core.ErrPrimitiveContract) {
			t.Fatalf("native refusal = %d/%v, want zero and both native and attest identities", n, nativeErr)
		}
		if n, err := writer.Write([]byte{2}); n != 0 || !errors.Is(err, nativeErr) {
			t.Fatalf("latched Write = %d/%v, want zero and original %v", n, err, nativeErr)
		}
		length, digest, err := writer.close(context.Canceled)
		if !errors.Is(err, nativeErr) || !errors.Is(err, context.Canceled) || length != (core.ByteCount{}) || digest != (core.SHA256Digest{}) {
			t.Fatalf("refused close = %v/%v/%v, want zero facts preserving native refusal and cancellation", length, digest, err)
		}
	})
	t.Run("empty stream produces no canonical proof", func(t *testing.T) {
		t.Parallel()
		writer := newCanonicalDigestWriter()
		length, digest, err := writer.close(nil)
		if !errors.Is(err, core.ErrAttestContract) || length != (core.ByteCount{}) || digest != (core.SHA256Digest{}) {
			t.Fatalf("empty close = %v/%v/%v, want zero facts and %v", length, digest, err, core.ErrAttestContract)
		}
	})
}
