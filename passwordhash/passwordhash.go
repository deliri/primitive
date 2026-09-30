// Package passwordhash executes caller-owned Argon2id policy through Go's
// maintained x/crypto implementation. It does not choose enrollment strength,
// encode stored credentials, obtain pepper secrets, or own account lifecycle.
package passwordhash

import (
	"context"
	"crypto/subtle"
	"errors"

	"github.com/deliri/primitive/v2026/core"
	"golang.org/x/crypto/argon2"
)

// Parameters are Argon2id v19's native cost inputs. Product security floors
// belong to the caller. Memory is expressed in KiB, not bytes.
type Parameters struct {
	MemoryKiB   uint32
	Iterations  uint32
	Parallelism uint8
}

func (p Parameters) Validate() error {
	if p.Iterations == 0 || p.Parallelism == 0 || p.MemoryKiB < 8*uint32(p.Parallelism) || uint64(p.MemoryKiB)*1024 > uint64(^uint(0)>>1) {
		return core.ErrPasswordHashContract
	}
	return nil
}

// Limits are explicit caller admission policy, not library-selected costs.
// One Deriver is shared by all callers in a resource domain. There is no queue:
// saturated admission returns ErrPasswordHashCapacity without performing a KDF.
type Limits struct {
	MemoryKiB       uint32
	Iterations      uint32
	Parallelism     uint8
	KeyBytes        uint32
	MaterialBytes   uint32
	SaltBytes       uint32
	ConcurrentCalls uint16
}

func (l Limits) Validate() error {
	if l.MemoryKiB < 8 || l.Iterations == 0 || l.Parallelism == 0 || l.KeyBytes == 0 || l.MaterialBytes == 0 || l.SaltBytes == 0 || l.ConcurrentCalls == 0 || uint64(l.MemoryKiB)*1024 > uint64(^uint(0)>>1) || uint64(l.KeyBytes) > uint64(^uint(0)>>1) {
		return core.ErrPasswordHashContract
	}
	return nil
}

// Request borrows its byte slices for one synchronous call. Material is the
// exact caller-owned password/pepper agreement, already assembled by its owner.
// Callers must not mutate either slice until the call returns.
type Request struct {
	Parameters Parameters
	Material   []byte
	Salt       []byte
	KeyBytes   uint32
}

func (r Request) Validate(limits Limits) error {
	if err := limits.Validate(); err != nil {
		return err
	}
	if err := r.Parameters.Validate(); err != nil {
		return err
	}
	if r.Parameters.MemoryKiB > limits.MemoryKiB || r.Parameters.Iterations > limits.Iterations || r.Parameters.Parallelism > limits.Parallelism || r.KeyBytes == 0 || r.KeyBytes > limits.KeyBytes || uint64(len(r.Material)) > uint64(limits.MaterialBytes) || len(r.Salt) == 0 || uint64(len(r.Salt)) > uint64(limits.SaltBytes) {
		return core.ErrPasswordHashContract
	}
	return nil
}

// Deriver owns only bounded admission. Copying the pointer shares that budget.
// No goroutines, timers, retries, network calls or persistent state are created.
type Deriver struct {
	limits Limits
	slots  chan struct{}
}

func New(limits Limits) (*Deriver, error) {
	if err := limits.Validate(); err != nil {
		return nil, err
	}
	return &Deriver{limits: limits, slots: make(chan struct{}, limits.ConcurrentCalls)}, nil
}

func (d *Deriver) Validate() error {
	if d == nil || d.slots == nil || cap(d.slots) != int(d.limits.ConcurrentCalls) {
		return core.ErrPasswordHashContract
	}
	return d.limits.Validate()
}

// Derive returns caller-owned key bytes. The caller must clear them after use.
// Argon2 is synchronous and cannot be interrupted mid-derivation. Cancellation
// is checked before admission and after completion; a cancelled call clears its
// result and returns no key. The caller's work and concurrency caps remain active
// until the native operation has completed.
func (d *Deriver) Derive(ctx context.Context, request Request) ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, err
	}
	if err := request.Validate(d.limits); err != nil {
		return nil, err
	}
	if ctx == nil {
		return nil, errors.Join(core.ErrPasswordHashContract, core.ErrNilContext)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	select {
	case d.slots <- struct{}{}:
		defer func() { <-d.slots }()
	default:
		return nil, core.ErrPasswordHashCapacity
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	parameters := request.Parameters
	key := argon2.IDKey(request.Material, request.Salt, parameters.Iterations, parameters.MemoryKiB, parameters.Parallelism, request.KeyBytes)
	if err := ctx.Err(); err != nil {
		clear(key)
		return nil, err
	}
	return key, nil
}

// Verify compares one exact-length digest in constant time, and always clears
// the derived candidate. A non-matching digest is false/nil, not an input error.
func (d *Deriver) Verify(ctx context.Context, request Request, digest []byte) (bool, error) {
	if uint64(len(digest)) != uint64(request.KeyBytes) {
		return false, core.ErrPasswordHashContract
	}
	key, err := d.Derive(ctx, request)
	if err != nil {
		return false, err
	}
	defer clear(key)
	return subtle.ConstantTimeCompare(key, digest) == 1, nil
}
