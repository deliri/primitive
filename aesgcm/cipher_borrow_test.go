package aesgcm

import (
	"bytes"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
)

func TestAESGCMBorrowOwnsIndependentAdmittedLifetime(t *testing.T) {
	t.Parallel()
	parent := fixtureCipher(t, make([]byte, core.AES256KeyBytes))
	alias := parent
	borrow, err := parent.Borrow()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := borrow.Destroy(); err != nil {
			t.Error(err)
		}
	}()
	borrowAlias := borrow
	if err := parent.Destroy(); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(parent.Validate(), core.ErrAESGCMContract) || !errors.Is(alias.Validate(), core.ErrAESGCMContract) {
		t.Fatalf("source/alias Validate() = (%v,%v), want both destroyed", parent.Validate(), alias.Validate())
	}
	plaintext, additional := []byte("provider grant issued before shutdown"), []byte("owner:calendar")
	sealed, err := borrow.Seal(SealRequest{Plaintext: plaintext, AdditionalData: additional})
	if err != nil {
		t.Fatalf("admitted borrow Seal() = %v, want usable after source destruction", err)
	}
	fromNative, err := nativeGCM(t, make([]byte, core.AES256KeyBytes)).Open(nil, sealed.Ciphertext[:core.AESGCMNonceBytes], sealed.Ciphertext[core.AESGCMNonceBytes:], additional)
	if err != nil || !bytes.Equal(fromNative, plaintext) {
		t.Fatalf("native Open() = (%q,%v), want exact admitted plaintext", fromNative, err)
	}
	clear(fromNative)
	if err := borrow.Destroy(); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(borrow.Validate(), core.ErrAESGCMContract) || !errors.Is(borrowAlias.Validate(), core.ErrAESGCMContract) {
		t.Fatalf("borrow/alias Validate() = (%v,%v), want both destroyed", borrow.Validate(), borrowAlias.Validate())
	}
	if !errors.Is(parent.Validate(), core.ErrAESGCMContract) {
		t.Fatalf("parent.Validate() = %v, want still destroyed after borrow destruction", parent.Validate())
	}
}

func TestAESGCMBorrowDestructionDoesNotRevokeSourceOrSibling(t *testing.T) {
	t.Parallel()
	parent := fixtureCipher(t, make([]byte, core.AES256KeyBytes))
	first, err := parent.Borrow()
	if err != nil {
		t.Fatal(err)
	}
	second, err := parent.Borrow()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := second.Destroy(); err != nil {
			t.Error(err)
		}
	}()
	if err := first.Destroy(); err != nil {
		t.Fatal(err)
	}
	for _, c := range []Cipher{parent, second} {
		sealed, err := c.Seal(SealRequest{Plaintext: []byte("sibling survives")})
		if err != nil || len(sealed.Ciphertext) != len("sibling survives")+core.AESGCMOverheadBytes {
			t.Fatalf("sibling/source Seal() = (%d,%v), want exact valid native ciphertext", len(sealed.Ciphertext), err)
		}
	}
}

func TestAESGCMBorrowRefusesZeroAndDestroyedWithoutCapability(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"zero source", "destroyed source"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var source Cipher
			if name == "destroyed source" {
				source = fixtureCipher(t, make([]byte, core.AES256KeyBytes))
				if err := source.Destroy(); err != nil {
					t.Fatal(err)
				}
			}
			got, err := source.Borrow()
			if !errors.Is(err, core.ErrAESGCMContract) || got != (Cipher{}) {
				t.Fatalf("Borrow() = (%v,%v), want zero/typed refusal", got, err)
			}
		})
	}
}

func TestAESGCMBorrowConcurrentSourceDestroyKeepsEveryAdmittedBorrowUsable(t *testing.T) {
	t.Parallel()
	parent := fixtureCipher(t, make([]byte, core.AES256KeyBytes))
	// One admitted borrower is pinned before the competing source destruction;
	// remaining callers pressure both allowed linearizations under the race detector.
	first, err := parent.Borrow()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := first.Destroy(); err != nil {
			t.Error(err)
		}
	}()
	const borrowers = 24
	start := make(chan struct{})
	results := make(chan error, borrowers+1)
	var workers sync.WaitGroup
	workers.Add(borrowers + 1)
	for range borrowers {
		go func() {
			defer workers.Done()
			<-start
			borrow, err := parent.Borrow()
			if err != nil {
				if !errors.Is(err, core.ErrAESGCMContract) || borrow != (Cipher{}) {
					results <- core.ErrAESGCMAuthentication
					return
				}
				results <- nil
				return
			}
			sealed, sealErr := borrow.Seal(SealRequest{Plaintext: []byte("concurrent grant")})
			if sealErr == nil {
				opened, openErr := borrow.Open(OpenRequest{Ciphertext: sealed.Ciphertext})
				sealErr = openErr
				if openErr == nil && !bytes.Equal(opened.Plaintext, []byte("concurrent grant")) {
					sealErr = core.ErrAESGCMAuthentication
				}
				clear(opened.Plaintext)
			}
			results <- errors.Join(sealErr, borrow.Destroy())
		}()
	}
	go func() { defer workers.Done(); <-start; results <- parent.Destroy() }()
	close(start)
	done := make(chan struct{})
	go func() { workers.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatalf("borrow workers exceeded %v join backstop, want completed owned operations", 10*time.Second)
	}
	for range borrowers + 1 {
		if err := <-results; err != nil {
			t.Fatalf("concurrent borrow/Destroy = %v, want usable admitted borrow or typed zero refusal", err)
		}
	}
	sealed, err := first.Seal(SealRequest{Plaintext: []byte("pinned admitted grant")})
	if err != nil || len(sealed.Ciphertext) != len("pinned admitted grant")+core.AESGCMOverheadBytes {
		t.Fatalf("pinned borrow after joined Destroy = (%d,%v), want complete admitted sealing", len(sealed.Ciphertext), err)
	}
	late, err := parent.Borrow()
	if !errors.Is(err, core.ErrAESGCMContract) || late != (Cipher{}) {
		t.Fatalf("late Borrow() = (%v,%v), want zero after joined Destroy", late, err)
	}
}
