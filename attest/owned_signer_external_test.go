package attest_test

import (
	"bytes"
	"crypto"
	"crypto/ed25519"
	"errors"
	"io"
	"runtime"
	"strconv"
	"sync"
	"testing"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
)

type signerFailure uint8

func (signerFailure) Error() string { return "native signing refusal" }

type mutableSigner struct {
	key     ed25519.PrivateKey
	next    ed25519.PrivateKey
	cause   error
	panicAt bool
	scratch [64]byte
}

func (s *mutableSigner) Public() crypto.PublicKey {
	return s.key.Public()
}

func (s *mutableSigner) Sign(random io.Reader, message []byte, options crypto.SignerOpts) ([]byte, error) {
	if s.panicAt {
		panic(signerFailure(1))
	}
	if s.cause != nil {
		return []byte("withdrawn signature"), s.cause
	}
	if s.next != nil {
		s.key, s.next = s.next, nil
	}
	// A provider may use mutable scratch storage. The native capability must
	// serialize the complete operation, rather than merely copying its result.
	n := copy(s.scratch[:], message)
	runtime.Gosched()
	return s.key.Sign(random, s.scratch[:n], options)
}

func ownedSignerKey(value byte) ed25519.PrivateKey {
	return ed25519.NewKeyFromSeed(bytes.Repeat([]byte{value}, ed25519.SeedSize))
}

func TestOwnedSignerCopiesLocalMaterialAndInvalidatesEveryHandle(t *testing.T) {
	t.Parallel()
	for _, pointer := range []bool{false, true} {
		t.Run(strconv.FormatBool(pointer), func(t *testing.T) {
			t.Parallel()
			private := ownedSignerKey(7)
			public := private.Public().(ed25519.PublicKey)
			var input crypto.Signer = private
			if pointer {
				input = &private
			}
			signer, err := attest.NewSigner(attest.SignerRequest{Signer: input})
			if err != nil {
				t.Fatal(err)
			}
			copy := signer
			clear(private)
			message := []byte("independent local custody")
			signature, err := copy.Sign(nil, message, crypto.Hash(0))
			if err != nil || !ed25519.Verify(public, message, signature) {
				t.Fatalf("owned local signing after source clear: %v", err)
			}
			projection := signer.Public().(ed25519.PublicKey)
			clear(projection)
			if !bytes.Equal(copy.Public().(ed25519.PublicKey), public) {
				t.Fatal("public projection altered native identity")
			}
			if err := signer.Destroy(); err != nil {
				t.Fatal(err)
			}
			if err := copy.Validate(); !errors.Is(err, core.ErrAttestContract) {
				t.Fatalf("destroyed copy validated: %v", err)
			}
			got, err := copy.Sign(nil, message, crypto.Hash(0))
			if got != nil || !errors.Is(err, core.ErrAttestContract) || copy.Public() != nil {
				t.Fatalf("destroyed copy emitted signature: %t/%v", got != nil, err)
			}
			if err := copy.Destroy(); !errors.Is(err, core.ErrAttestContract) {
				t.Fatalf("destroyed copy closed again: %v", err)
			}
		})
	}
}

func TestOwnedSignerRejectsMalformedCapabilities(t *testing.T) {
	t.Parallel()
	for _, input := range []crypto.Signer{nil, (*ed25519.PrivateKey)(nil), ed25519.PrivateKey{}, ed25519.PrivateKey(make([]byte, 63)), ed25519.PrivateKey(make([]byte, 65))} {
		if err := (attest.SignerRequest{Signer: input}).Validate(); !errors.Is(err, core.ErrAttestContract) {
			t.Fatalf("malformed request validated: %v", err)
		}
		got, err := attest.NewSigner(attest.SignerRequest{Signer: input})
		if got != (attest.Signer{}) || !errors.Is(err, core.ErrAttestContract) {
			t.Fatalf("malformed request returned capability: %v/%v", got, err)
		}
	}
	zero := attest.Signer{}
	if err := zero.Validate(); !errors.Is(err, core.ErrAttestContract) || zero.Public() != nil {
		t.Fatalf("zero signer admitted: %v", err)
	}
	if err := zero.Destroy(); !errors.Is(err, core.ErrAttestContract) {
		t.Fatalf("zero signer destroyed: %v", err)
	}
}

func TestOwnedSignerRefusesProviderRotationBeforeAndDuringExecution(t *testing.T) {
	t.Parallel()
	for _, during := range []bool{false, true} {
		t.Run(strconv.FormatBool(during), func(t *testing.T) {
			t.Parallel()
			provider := &mutableSigner{key: ownedSignerKey(7)}
			signer, err := attest.NewSigner(attest.SignerRequest{Signer: provider})
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = signer.Destroy() }()
			public := signer.Public().(ed25519.PublicKey)
			if during {
				provider.next = ownedSignerKey(9)
			} else {
				provider.key = ownedSignerKey(9)
			}
			got, err := signer.Sign(nil, []byte("bound to the first identity"), crypto.Hash(0))
			if got != nil || !errors.Is(err, core.ErrAttestSignerChanged) || !errors.Is(err, core.ErrAttestVerification) {
				t.Fatalf("provider rotation emitted signature: %t/%v", got != nil, err)
			}
			if !bytes.Equal(public, signer.Public().(ed25519.PublicKey)) {
				t.Fatal("provider rotation rewrote the admitted public identity")
			}
		})
	}
}

func TestOwnedSignerPreservesNativeFailureAndSurvivesPanic(t *testing.T) {
	t.Parallel()
	provider := &mutableSigner{key: ownedSignerKey(7)}
	signer, err := attest.NewSigner(attest.SignerRequest{Signer: provider})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = signer.Destroy() }()
	message := []byte("sign after refusing")
	provider.cause = signerFailure(1)
	got, err := signer.Sign(nil, message, crypto.Hash(0))
	if got != nil || !errors.Is(err, signerFailure(1)) || !errors.Is(err, core.ErrAttestContract) {
		t.Fatalf("native refusal: signature=%t error=%v", got != nil, err)
	}
	provider.cause, provider.panicAt = nil, true
	got, err = signer.Sign(nil, message, crypto.Hash(0))
	if got != nil || !errors.Is(err, core.ErrAttestContract) || !errors.Is(err, signerFailure(1)) {
		t.Fatalf("native panic: signature=%t error=%v", got != nil, err)
	}
	provider.panicAt = false
	got, err = signer.Sign(nil, message, crypto.Hash(0))
	if err != nil || !ed25519.Verify(provider.key.Public().(ed25519.PublicKey), message, got) {
		t.Fatalf("native owner unusable after panic: %v", err)
	}
	got, err = signer.Sign(nil, message, nil)
	if got != nil || !errors.Is(err, core.ErrAttestContract) {
		t.Fatalf("nil options signed: %t/%v", got != nil, err)
	}
}

func TestOwnedSignerSerializesCopiedExternalCapabilities(t *testing.T) {
	t.Parallel()
	provider := &mutableSigner{key: ownedSignerKey(7)}
	signer, err := attest.NewSigner(attest.SignerRequest{Signer: provider})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = signer.Destroy() }()
	public := signer.Public().(ed25519.PublicKey)
	var workers sync.WaitGroup
	for worker := range 16 {
		workers.Go(func() {
			copy := signer
			for sequence := range 32 {
				message := []byte("worker " + strconv.Itoa(worker) + " sequence " + strconv.Itoa(sequence))
				signature, err := copy.Sign(nil, message, crypto.Hash(0))
				if err != nil || !ed25519.Verify(public, message, signature) {
					t.Errorf("signature crossed concurrent messages for %q: %v", message, err)
					return
				}
			}
		})
	}
	workers.Wait()
}

func FuzzOwnedSignerNativeMessageConservation(f *testing.F) {
	f.Add([]byte("signed bytes"), false)
	f.Add([]byte{}, false)
	f.Add([]byte("refused after destroy"), true)
	f.Fuzz(func(t *testing.T, message []byte, destroy bool) {
		private := ownedSignerKey(7)
		public := private.Public().(ed25519.PublicKey)
		signer, err := attest.NewSigner(attest.SignerRequest{Signer: private})
		clear(private)
		if err != nil {
			t.Fatal(err)
		}
		if destroy {
			if err := signer.Destroy(); err != nil {
				t.Fatal(err)
			}
		} else {
			defer func() { _ = signer.Destroy() }()
		}
		signature, err := signer.Sign(nil, message, crypto.Hash(0))
		if destroy {
			if signature != nil || !errors.Is(err, core.ErrAttestContract) {
				t.Fatalf("destroyed fuzz capability signed: %t/%v", signature != nil, err)
			}
			return
		}
		if err != nil || !ed25519.Verify(public, message, signature) {
			t.Fatalf("native signature does not conserve message: %v", err)
		}
	})
}
