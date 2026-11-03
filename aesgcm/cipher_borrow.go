package aesgcm

import "github.com/deliri/primitive/v2026/core"

// Borrow admits an independently owned operation lifetime while the source is
// live. Ordinary source copies still share destruction. A successful borrow
// retains the same immutable native key schedule until its own Destroy, so
// source shutdown cannot strand an already admitted operation between an
// external effect and sealing its result. No key bytes are exported or copied.
//
// The caller must destroy every successful borrow on every outcome. Native
// nonce limits continue to apply to the shared key across all borrows.
func (c Cipher) Borrow() (Cipher, error) {
	if c.state == nil {
		return Cipher{}, core.ErrAESGCMContract
	}
	c.state.mutex.RLock()
	defer c.state.mutex.RUnlock()
	if c.state.native == nil {
		return Cipher{}, core.ErrAESGCMContract
	}
	return Cipher{state: &cipherState{native: c.state.native}}, nil
}
