package passwordhash

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"testing/synctest"
	"time"

	"golang.org/x/crypto/argon2"
)

// Exhaust the two public operations and the three waiter outcomes. The real
// admission channel is held explicitly; native Argon2 still produces the key.
// synctest.Wait proves the waiter is blocked, without a timing-based guess.
func TestPasswordAdmissionQueueLayerTriad(t *testing.T) {
	t.Parallel()
	for _, operation := range []struct {
		name   string
		verify bool
	}{{name: "derive"}, {name: "verify", verify: true}} {
		for _, ending := range []struct {
			name     string
			cancel   bool
			deadline bool
			wantErr  error
		}{
			{name: "released slot completes native work"},
			{name: "cancelled waiter leaves occupied slot unchanged", cancel: true, wantErr: context.Canceled},
			{name: "expired waiter leaves occupied slot unchanged", deadline: true, wantErr: context.DeadlineExceeded},
		} {
			t.Run(operation.name+"/"+ending.name, func(t *testing.T) {
				t.Parallel()
				synctest.Test(t, func(t *testing.T) {
					engine, err := New(fixtureLimits())
					if err != nil {
						t.Fatal(err)
					}
					request := fixtureRequest()
					want := argon2.IDKey(request.Material, request.Salt, request.Parameters.Iterations, request.Parameters.MemoryKiB, request.Parameters.Parallelism, request.KeyBytes)
					defer clear(want)
					ctx, cancel := context.WithCancel(t.Context())
					if ending.deadline {
						cancel()
						ctx, cancel = context.WithTimeout(t.Context(), time.Second)
					}
					engine.slots <- struct{}{}
					type result struct {
						key     []byte
						err     error
						matched bool
					}
					outcome, done := make(chan result, 1), make(chan struct{})
					go func() {
						defer close(done)
						var got result
						if operation.verify {
							got.matched, got.err = engine.Verify(ctx, request, want)
						} else {
							got.key, got.err = engine.Derive(ctx, request)
						}
						outcome <- got
					}()
					t.Cleanup(func() {
						cancel()
						select {
						case <-done:
						case <-time.After(10 * time.Second):
							t.Error("owned waiter = blocked, want exited")
						}
					})
					synctest.Wait()
					select {
					case got := <-outcome:
						clear(got.key)
						t.Fatalf("occupied admission = returned %v, want waiting for release or cancellation", got.err)
					default:
					}
					if got := len(engine.slots); got != 1 {
						t.Fatalf("occupied slots = %d, want 1 while waiter is blocked", got)
					}
					if ending.cancel {
						cancel()
					} else if !ending.deadline {
						<-engine.slots
					}
					var got result
					select {
					case got = <-outcome:
					case <-time.After(10 * time.Second):
						t.Fatal("waiter = blocked, want terminal outcome")
					}
					defer clear(got.key)
					if ending.wantErr != nil {
						if got.key != nil || got.matched || !errors.Is(got.err, ending.wantErr) || len(engine.slots) != 1 {
							t.Fatalf("refused waiter = key:%d match:%t error:%v slots:%d, want no result/%v/occupied owner", len(got.key), got.matched, got.err, len(engine.slots), ending.wantErr)
						}
						<-engine.slots
					} else if got.err != nil || len(engine.slots) != 0 || (operation.verify && !got.matched) || (!operation.verify && !bytes.Equal(got.key, want)) {
						t.Fatalf("released waiter = key:%d match:%t error:%v slots:%d, want native result/nil/released", len(got.key), got.matched, got.err, len(engine.slots))
					}
					key, err := engine.Derive(t.Context(), request)
					defer clear(key)
					if err != nil || !bytes.Equal(key, want) || len(engine.slots) != 0 {
						t.Fatalf("reused admission = key:%d error:%v slots:%d, want native result/nil/released", len(key), err, len(engine.slots))
					}
				})
			})
		}
	}
}
