package keygen_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/keygen"
)

func TestPublicKeyHexStreamPreservesCanonicalTokenWithoutWhitespaceQuota(t *testing.T) {
	t.Parallel()
	key, _ := openSSHFixture(t, "", keygen.OpenSSHPrivateKeyLabel)
	want, err := key.PublicKey()
	if err != nil {
		t.Fatal(err)
	}
	hex, err := want.Hex()
	if err != nil {
		t.Fatal(err)
	}
	for _, padding := range []string{"", "\t\r\n ", "\u2003\u0085\u00a0", strings.Repeat(" ", 1048577)} {
		got, err := keygen.ReadPublicKeyHex(t.Context(), keygen.PublicKeyHexReadRequest{Source: strings.NewReader(padding + hex + padding)})
		if err != nil || got != want {
			t.Fatalf("whitespace extent=%d changed admitted identity: %v", len(padding), err)
		}
	}
}

func TestPublicKeyHexStreamRefusesMalformedTokensAndNativeSourceFailure(t *testing.T) {
	t.Parallel()
	key, _ := openSSHFixture(t, "", keygen.OpenSSHPrivateKeyLabel)
	public, err := key.PublicKey()
	if err != nil {
		t.Fatal(err)
	}
	hex, err := public.Hex()
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"", " \n\t", hex[:63], hex + "0", strings.ToUpper(hex), "g" + hex[1:], hex[:32] + " " + hex[32:], hex + "\n" + hex, "\ufeff" + hex, "\xff" + hex, hex + "\x00"} {
		got, err := keygen.ReadPublicKeyHex(t.Context(), keygen.PublicKeyHexReadRequest{Source: strings.NewReader(text)})
		if got != (core.Ed25519PublicKey{}) || !errors.Is(err, core.ErrKeygenPublicKeyText) {
			t.Fatalf("malformed token produced public identity: %t/%v", got != (core.Ed25519PublicKey{}), err)
		}
	}
	for _, source := range []io.Reader{nil, (*bytes.Reader)(nil), iotest.ErrReader(io.ErrClosedPipe), io.MultiReader(strings.NewReader(hex+"\n"), iotest.ErrReader(io.ErrClosedPipe))} {
		got, err := keygen.ReadPublicKeyHex(t.Context(), keygen.PublicKeyHexReadRequest{Source: source})
		if got != (core.Ed25519PublicKey{}) || !errors.Is(err, core.ErrKeygenContract) {
			t.Fatalf("refused source produced public identity: %t/%v", got != (core.Ed25519PublicKey{}), err)
		}
		if source != nil && source != (*bytes.Reader)(nil) && !errors.Is(err, io.ErrClosedPipe) {
			t.Fatalf("native source failure was erased: %v", err)
		}
		if errors.Is(err, core.ErrKeygenPublicKeyText) {
			t.Fatalf("source refusal was misclassified as malformed text: %v", err)
		}
	}
}

type cancelingPublicKeyReader struct {
	cancel context.CancelCauseFunc
	text   string
}

func (r cancelingPublicKeyReader) Read(p []byte) (int, error) {
	r.cancel(io.ErrNoProgress)
	return copy(p, r.text), io.ErrClosedPipe
}

func TestPublicKeyHexStreamWithdrawsCancellationDuringNativeRead(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancelCause(t.Context())
	defer cancel(nil)
	got, err := keygen.ReadPublicKeyHex(ctx, keygen.PublicKeyHexReadRequest{Source: cancelingPublicKeyReader{cancel: cancel, text: "not a key"}})
	if got != (core.Ed25519PublicKey{}) || !errors.Is(err, context.Canceled) || !errors.Is(err, io.ErrNoProgress) || !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("native read cancellation lost causes or emitted identity: %t/%v", got != (core.Ed25519PublicKey{}), err)
	}
	got, err = keygen.ReadPublicKeyHex(nil, keygen.PublicKeyHexReadRequest{Source: strings.NewReader("")})
	if got != (core.Ed25519PublicKey{}) || !errors.Is(err, core.ErrNilContext) {
		t.Fatalf("nil context admitted: %v", err)
	}
}

func FuzzPublicKeyHexStreamMatchesNativeTextAdmission(f *testing.F) {
	f.Add([]byte(strings.Repeat("a", 64)))
	f.Add([]byte(" \n\t"))
	f.Add([]byte("\u2003" + strings.Repeat("a", 64) + "\u0085"))
	f.Fuzz(func(t *testing.T, data []byte) {
		var want core.Ed25519PublicKey
		wantErr := want.UnmarshalText(bytes.TrimSpace(data))
		got, err := keygen.ReadPublicKeyHex(t.Context(), keygen.PublicKeyHexReadRequest{Source: bytes.NewReader(data)})
		if wantErr != nil {
			if got != (core.Ed25519PublicKey{}) || !errors.Is(err, core.ErrKeygenContract) {
				t.Fatalf("native text refusal emitted a streamed identity: %v", err)
			}
		} else if err != nil || got != want {
			t.Fatalf("streamed identity differs from native text admission: %v", err)
		}
	})
}
