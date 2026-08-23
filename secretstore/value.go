package secretstore

import (
	"errors"
	"fmt"
	"io"
	"sync"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
)

const (
	// PayloadMaximumBytes is Google Secret Manager's exact payload ceiling.
	PayloadMaximumBytes = 64 * 1024
)

type valueState struct {
	bytes     [PayloadMaximumBytes]byte
	mu        sync.RWMutex
	extent    int
	destroyed bool
}

// Value is bounded secret material whose copies share destruction state.
type Value struct{ state *valueState }

// NewValue copies one admitted payload into fixed-capacity secret custody.
func NewValue(payload []byte) (Value, error) {
	if err := validatePayload(payload); err != nil {
		return Value{}, err
	}
	state := &valueState{extent: len(payload)}
	copy(state.bytes[:], payload)
	return Value{state: state}, nil
}

func validatePayload(payload []byte) error {
	if len(payload) > PayloadMaximumBytes {
		return errors.Join(core.ErrSecretStorePayload, core.ErrSecretStoreContract)
	}
	return nil
}

// Validate rejects unset or destroyed secret material.
func (v Value) Validate() error {
	if v.state == nil {
		return errors.Join(core.ErrSecretStorePayload, core.ErrSecretStoreContract)
	}
	v.state.mu.RLock()
	defer v.state.mu.RUnlock()
	if !valueStateValid(v.state) {
		return errors.Join(core.ErrSecretStorePayload, core.ErrSecretStoreContract)
	}
	return nil
}

// CopyBytes explicitly projects an independent caller-owned copy. Destroy
// cannot clear copies already returned to a caller.
func (v Value) CopyBytes() ([]byte, error) {
	if v.state == nil {
		return nil, errors.Join(core.ErrSecretStorePayload, core.ErrSecretStoreContract)
	}
	v.state.mu.RLock()
	defer v.state.mu.RUnlock()
	if !valueStateValid(v.state) {
		return nil, errors.Join(core.ErrSecretStorePayload, core.ErrSecretStoreContract)
	}
	result := make([]byte, v.state.extent)
	copy(result, v.state.bytes[:v.state.extent])
	return result, nil
}

// Text explicitly projects UTF-8 secret text for a caller that owns that
// semantic requirement. Opaque non-text payloads remain valid Values.
func (v Value) Text() (string, error) {
	if v.state == nil {
		return "", errors.Join(core.ErrSecretStorePayload, core.ErrSecretStoreContract)
	}
	v.state.mu.RLock()
	defer v.state.mu.RUnlock()
	if !valueStateValid(v.state) || !utf8.Valid(v.state.bytes[:v.state.extent]) {
		return "", errors.Join(core.ErrSecretStorePayload, core.ErrSecretStoreContract)
	}
	return string(v.state.bytes[:v.state.extent]), nil
}

// Destroy clears the shared payload. Repeated destruction is a no-op.
func (v Value) Destroy() error {
	if v.state == nil {
		return errors.Join(core.ErrSecretStorePayload, core.ErrSecretStoreContract)
	}
	v.state.mu.Lock()
	defer v.state.mu.Unlock()
	if v.state.destroyed {
		return nil
	}
	clear(v.state.bytes[:])
	v.state.extent = 0
	v.state.destroyed = true
	return nil
}

// Format keeps every generic formatting verb secret-free.
func (v Value) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, core.RedactedValueText)
}

func valueStateValid(state *valueState) bool {
	return !state.destroyed && state.extent >= 0 && state.extent <= PayloadMaximumBytes
}
