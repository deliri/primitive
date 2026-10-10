package passwordhash

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"math"
	"reflect"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"golang.org/x/crypto/argon2"
)

// Tiny native costs keep mechanical tests bounded. They are not recommended
// product password policy. Kernel's production 64 MiB policy has separate proof.
func fixtureLimits() Limits {
	return Limits{MemoryKiB: 32, Iterations: 2, Parallelism: 2, KeyBytes: 32, MaterialBytes: 64, SaltBytes: 32, ConcurrentCalls: 1}
}

func fixtureRequest() Request {
	return Request{Parameters: Parameters{MemoryKiB: 16, Iterations: 1, Parallelism: 1}, Material: []byte("exact-password-material"), Salt: []byte("a-public-salt"), KeyBytes: 32}
}

func TestArgon2idAdmissionBoundariesLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		mutate func(*Request)
		want   bool
	}{
		{"native memory below one-lane floor", func(r *Request) { r.Parameters.MemoryKiB = 7 }, false},
		{"native memory at one-lane floor", func(r *Request) { r.Parameters.MemoryKiB = 8 }, true},
		{"native memory above one-lane floor", func(r *Request) { r.Parameters.MemoryKiB = 9 }, true},
		{"memory below configured ceiling", func(r *Request) { r.Parameters.MemoryKiB = 31 }, true},
		{"memory at configured ceiling", func(r *Request) { r.Parameters.MemoryKiB = 32 }, true},
		{"memory above configured ceiling", func(r *Request) { r.Parameters.MemoryKiB = 33 }, false},
		{"memory at native integer extreme", func(r *Request) { r.Parameters.MemoryKiB = math.MaxUint32 }, false},
		{"zero iteration count", func(r *Request) { r.Parameters.Iterations = 0 }, false},
		{"iteration below configured ceiling", func(r *Request) { r.Parameters.Iterations = 1 }, true},
		{"iteration at configured ceiling", func(r *Request) { r.Parameters.Iterations = 2 }, true},
		{"iteration above configured ceiling", func(r *Request) { r.Parameters.Iterations = 3 }, false},
		{"iteration at native integer extreme", func(r *Request) { r.Parameters.Iterations = math.MaxUint32 }, false},
		{"zero parallelism", func(r *Request) { r.Parameters.Parallelism = 0 }, false},
		{"parallelism at configured ceiling", func(r *Request) { r.Parameters.Parallelism = 2 }, true},
		{"parallelism above configured ceiling", func(r *Request) { r.Parameters.Parallelism = 3; r.Parameters.MemoryKiB = 32 }, false},
		{"memory cannot silently grow to match lanes", func(r *Request) { r.Parameters.Parallelism = 2; r.Parameters.MemoryKiB = 15 }, false},
		{"empty key extent", func(r *Request) { r.KeyBytes = 0 }, false},
		{"smallest key extent", func(r *Request) { r.KeyBytes = 1 }, true},
		{"key below configured extent", func(r *Request) { r.KeyBytes = 31 }, true},
		{"key at configured extent", func(r *Request) { r.KeyBytes = 32 }, true},
		{"key above configured extent", func(r *Request) { r.KeyBytes = 33 }, false},
		{"key native integer extreme", func(r *Request) { r.KeyBytes = math.MaxUint32 }, false},
		{"empty material is a native KDF input", func(r *Request) { r.Material = nil }, true},
		{"material below caller bound", func(r *Request) { r.Material = bytes.Repeat([]byte{0x31}, 63) }, true},
		{"material at caller bound", func(r *Request) { r.Material = bytes.Repeat([]byte{0x31}, 64) }, true},
		{"material above caller bound", func(r *Request) { r.Material = bytes.Repeat([]byte{0x31}, 65) }, false},
		{"absent salt", func(r *Request) { r.Salt = nil }, false},
		{"salt below caller bound", func(r *Request) { r.Salt = bytes.Repeat([]byte{0x52}, 31) }, true},
		{"salt at caller bound", func(r *Request) { r.Salt = bytes.Repeat([]byte{0x52}, 32) }, true},
		{"salt above caller bound", func(r *Request) { r.Salt = bytes.Repeat([]byte{0x52}, 33) }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			request := fixtureRequest()
			tc.mutate(&request)
			material, salt := bytes.Clone(request.Material), bytes.Clone(request.Salt)
			engine, err := New(fixtureLimits())
			if err != nil {
				t.Fatal(err)
			}
			key, err := engine.Derive(t.Context(), request)
			defer clear(key)
			if !tc.want {
				if key != nil || !errors.Is(err, core.ErrPasswordHashContract) {
					t.Fatalf("refused derivation = %d/%v, want nil/contract", len(key), err)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				want := argon2.IDKey(request.Material, request.Salt, request.Parameters.Iterations, request.Parameters.MemoryKiB, request.Parameters.Parallelism, request.KeyBytes)
				defer clear(want)
				if !bytes.Equal(key, want) {
					t.Fatal("derived key = different bytes, want direct Go Argon2id answer")
				}
			}
			if !bytes.Equal(request.Material, material) || !bytes.Equal(request.Salt, salt) || len(engine.slots) != 0 {
				t.Fatalf("input preservation = material:%t salt:%t slots:%d, want true/true/0", bytes.Equal(request.Material, material), bytes.Equal(request.Salt, salt), len(engine.slots))
			}
		})
	}
}

func TestArgon2idConstructorRefusesMissingLimits(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		mutate func(*Limits)
	}{
		{"memory below native floor", func(l *Limits) { l.MemoryKiB = 7 }},
		{"missing iterations", func(l *Limits) { l.Iterations = 0 }},
		{"missing parallelism", func(l *Limits) { l.Parallelism = 0 }},
		{"missing key bound", func(l *Limits) { l.KeyBytes = 0 }},
		{"missing material bound", func(l *Limits) { l.MaterialBytes = 0 }},
		{"missing salt bound", func(l *Limits) { l.SaltBytes = 0 }},
		{"missing concurrency bound", func(l *Limits) { l.ConcurrentCalls = 0 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			limits := fixtureLimits()
			tc.mutate(&limits)
			got, err := New(limits)
			if got != nil || !errors.Is(err, core.ErrPasswordHashContract) {
				t.Fatalf("New = %p/%v, want nil/contract", got, err)
			}
		})
	}
}

func TestArgon2idAdmissionOwnershipLayerTriad(t *testing.T) {
	t.Parallel()
	engine, err := New(fixtureLimits())
	if err != nil {
		t.Fatal(err)
	}
	request := fixtureRequest()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	key, err := engine.Derive(ctx, request)
	if key != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled saturated derivation = %d/%v, want nil/cancelled", len(key), err)
	}
	key, err = engine.Derive(t.Context(), request)
	defer clear(key)
	if err != nil || len(key) != int(request.KeyBytes) || len(engine.slots) != 0 {
		t.Fatalf("released derivation = %d/%v slots=%d, want %d/nil/0", len(key), err, len(engine.slots), request.KeyBytes)
	}
	for _, invalid := range []*Deriver{nil, {}} {
		key, err := invalid.Derive(t.Context(), request)
		if key != nil || !errors.Is(err, core.ErrPasswordHashContract) {
			t.Fatalf("zero engine = %d/%v, want nil/contract", len(key), err)
		}
	}
	//lint:ignore SA1012 Deliberately exercise the public nil-context rejection boundary.
	key, err = engine.Derive(nil, request)
	if key != nil || !errors.Is(err, core.ErrNilContext) {
		t.Fatalf("nil context = %d/%v, want nil/nil-context", len(key), err)
	}
}

func TestArgon2idVerificationLayerTriad(t *testing.T) {
	t.Parallel()
	engine, err := New(fixtureLimits())
	if err != nil {
		t.Fatal(err)
	}
	request := fixtureRequest()
	key, err := engine.Derive(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { clear(key) })
	foreign := bytes.Clone(key)
	foreign[0] ^= 1
	for _, tc := range []struct {
		name    string
		digest  []byte
		want    bool
		wantErr error
	}{
		{"issued digest matches", bytes.Clone(key), true, nil},
		{"same width foreign digest refuses authentication", foreign, false, nil},
		{"missing digest refuses contract", nil, false, core.ErrPasswordHashContract},
		{"short digest refuses contract", key[:len(key)-1], false, core.ErrPasswordHashContract},
		{"long digest refuses contract", append(bytes.Clone(key), 0), false, core.ErrPasswordHashContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			local, err := New(fixtureLimits())
			if err != nil {
				t.Fatal(err)
			}
			got, err := local.Verify(t.Context(), request, tc.digest)
			if got != tc.want || !errors.Is(err, tc.wantErr) {
				t.Fatalf("Verify = %t/%v, want %t/%v", got, err, tc.want, tc.wantErr)
			}
		})
	}
}

func FuzzArgon2idRequestSemanticClosure(f *testing.F) {
	seed := fixtureRequest()
	limits := fixtureLimits()
	if err := seed.Validate(limits); err != nil {
		f.Fatal(err)
	}
	f.Add(seed.Parameters.MemoryKiB, seed.Parameters.Iterations, seed.Parameters.Parallelism, seed.KeyBytes, seed.Material, seed.Salt)
	f.Add(uint32(7), uint32(1), uint8(1), uint32(32), []byte{}, []byte{1})
	f.Add(uint32(16), uint32(1), uint8(1), uint32(0), []byte{1}, []byte{2})
	f.Fuzz(func(t *testing.T, memory, iterations uint32, parallelism uint8, extent uint32, material, salt []byte) {
		request := Request{Parameters: Parameters{MemoryKiB: memory, Iterations: iterations, Parallelism: parallelism}, Material: material, Salt: salt, KeyBytes: extent}
		engine, err := New(limits)
		if err != nil {
			t.Fatal(err)
		}
		key, err := engine.Derive(t.Context(), request)
		defer clear(key)
		want := memory >= 8*uint32(parallelism) && memory <= 32 && iterations >= 1 && iterations <= 2 && parallelism >= 1 && parallelism <= 2 && extent >= 1 && extent <= 32 && len(material) <= 64 && len(salt) >= 1 && len(salt) <= 32
		if !want {
			if key != nil || !errors.Is(err, core.ErrPasswordHashContract) {
				t.Fatalf("refused request returned key=%d err=%v", len(key), err)
			}
			return
		}
		if err != nil {
			t.Fatal(err)
		}
		direct := argon2.IDKey(material, salt, iterations, memory, parallelism, extent)
		defer clear(direct)
		if !bytes.Equal(key, direct) {
			t.Fatal("accepted key = different bytes, want native Argon2id answer")
		}
		matched, err := engine.Verify(t.Context(), request, direct)
		if err != nil || !matched {
			t.Fatalf("issued key = matched:%t/%v, want true/nil", matched, err)
		}
		direct[0] ^= 1
		matched, err = engine.Verify(t.Context(), request, direct)
		if err != nil || matched {
			t.Fatalf("one-bit foreign key = matched:%t/%v, want false/nil", matched, err)
		}
	})
}

func TestPasswordHashDataFlowInventory(t *testing.T) {
	t.Parallel()
	inventory := []struct {
		nominal reflect.Type
		role    string
	}{
		{reflect.TypeFor[Parameters](), "native cost agreement"}, {reflect.TypeFor[Limits](), "caller admission policy"},
		{reflect.TypeFor[Request](), "borrowed operation input"}, {reflect.TypeFor[Deriver](), "shared admission capability"},
	}
	file, err := parser.ParseFile(token.NewFileSet(), "passwordhash.go", nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	var got, want []string
	for _, entry := range inventory {
		want = append(want, entry.nominal.Name())
		if entry.role == "" {
			t.Fatal("inventory role = empty, want explicit ownership role")
		}
	}
	ast.Inspect(file, func(node ast.Node) bool {
		if spec, ok := node.(*ast.TypeSpec); ok {
			if _, ok := spec.Type.(*ast.StructType); ok {
				got = append(got, spec.Name.Name)
			}
		}
		return true
	})
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("production structs=%v, want classified %v", got, want)
	}
}

// errBoundaryContext is a test-only synchronization seam at actual context
// checks; the wrapped context still supplies every cancellation/error fact.
// It does not replace the KDF or alter the executor's production code.
type errBoundaryContext struct {
	context.Context
	onErr func()
}

func (c errBoundaryContext) Err() error { c.onErr(); return c.Context.Err() }

func TestArgon2idConcurrentAdmissionAndRelease(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		const calls = 4
		const capacity = 2
		limits := fixtureLimits()
		limits.ConcurrentCalls = capacity
		engine, err := New(limits)
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		entered := make(chan struct{}, calls)
		release := make(chan struct{})
		type outcome struct {
			key []byte
			err error
		}
		outcomes := make(chan outcome, calls)
		var workers sync.WaitGroup
		workers.Add(calls)
		for range calls {
			go func() {
				defer workers.Done()
				var checks atomic.Int32
				owned := errBoundaryContext{Context: ctx, onErr: func() {
					if checks.Add(1) == 2 {
						entered <- struct{}{}
						select {
						case <-release:
						case <-ctx.Done():
						}
					}
				}}
				key, err := engine.Derive(owned, fixtureRequest())
				outcomes <- outcome{key: key, err: err}
			}()
		}
		done := make(chan struct{})
		go func() { workers.Wait(); close(done) }()
		t.Cleanup(func() {
			cancel()
			select {
			case <-done:
			case <-time.After(10 * time.Second):
				t.Error("owned derivation workers = blocked, want all exited")
			}
		})
		for range capacity {
			select {
			case <-entered:
			case <-time.After(10 * time.Second):
				t.Fatal("admission = underfilled, want configured capacity before deadline")
			}
		}
		synctest.Wait()
		select {
		case got := <-outcomes:
			clear(got.key)
			t.Fatalf("occupied admission = returned %v, want all excess callers queued", got.err)
		default:
		}
		if got := len(engine.slots); got != capacity {
			t.Fatalf("in-flight admissions = %d, want %d", got, capacity)
		}
		close(release)
		request := fixtureRequest()
		want := argon2.IDKey(request.Material, request.Salt, request.Parameters.Iterations, request.Parameters.MemoryKiB, request.Parameters.Parallelism, request.KeyBytes)
		defer clear(want)
		for range calls {
			select {
			case got := <-outcomes:
				matched := bytes.Equal(got.key, want)
				clear(got.key)
				if got.err != nil || !matched {
					t.Fatalf("concurrent native derivation = matched:%t/%v, want true/nil", matched, got.err)
				}
			case <-time.After(10 * time.Second):
				t.Fatal("admitted native derivation = blocked, want completion before deadline")
			}
		}
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Fatal("owned workers = blocked, want all exited")
		}
		if got := len(engine.slots); got != 0 {
			t.Fatalf("completed admissions = %d, want 0", got)
		}
		matched, err := engine.Verify(t.Context(), request, want)
		if err != nil || !matched {
			t.Fatalf("reused capacity = %t/%v, want true/nil", matched, err)
		}
	})
}

func TestArgon2idCancellationAtEveryContextBoundary(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		cancelAt int32
	}{
		{"before admission", 1}, {"after admission before native work", 2}, {"after native work before publishing key", 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			engine, err := New(fixtureLimits())
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			var checks atomic.Int32
			owned := errBoundaryContext{Context: ctx, onErr: func() {
				if checks.Add(1) == tc.cancelAt {
					cancel()
				}
			}}
			got, err := engine.Derive(owned, fixtureRequest())
			if got != nil || !errors.Is(err, context.Canceled) || checks.Load() != tc.cancelAt || len(engine.slots) != 0 {
				clear(got)
				t.Fatalf("cancel at %d = key:%d/%v checks:%d slots:%d, want no key/cancelled/exact check/released", tc.cancelAt, len(got), err, checks.Load(), len(engine.slots))
			}
			got, err = engine.Derive(t.Context(), fixtureRequest())
			defer clear(got)
			if err != nil || len(got) != 32 {
				t.Fatalf("reuse after cancellation = %d/%v, want 32/nil", len(got), err)
			}
		})
	}
}

func TestArgon2idPublishedKnownAnswers(t *testing.T) {
	t.Parallel()
	// Go x/crypto's published Argon2id test vectors for password/somesalt.
	// Fixed answers also detect accidentally using Argon2i, changed cost units,
	// swapped password/salt, or truncating a caller-selected output extent.
	for _, tc := range []struct {
		name       string
		iterations uint32
		lanes      uint8
		wantHex    string
	}{
		{"single pass", 1, 1, "655ad15eac652dc59f7170a7332bf49b8469be1fdb9c28bb"},
		{"second pass", 2, 1, "068d62b26455936aa6ebe60060b0a65870dbfa3ddf8d41f7"},
		{"two lanes", 2, 2, "350ac37222f436ccb5c0972f1ebd3bf6b958bf2071841362"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			limits := fixtureLimits()
			limits.MemoryKiB = 64
			engine, err := New(limits)
			if err != nil {
				t.Fatal(err)
			}
			request := Request{Parameters: Parameters{MemoryKiB: 64, Iterations: tc.iterations, Parallelism: tc.lanes}, Material: []byte("password"), Salt: []byte("somesalt"), KeyBytes: 24}
			got, err := engine.Derive(t.Context(), request)
			defer clear(got)
			if err != nil || hex.EncodeToString(got) != tc.wantHex {
				t.Fatalf("published Argon2id vector = %x/%v, want %s/nil", got, err, tc.wantHex)
			}
		})
	}
}
