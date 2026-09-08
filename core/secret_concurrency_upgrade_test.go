package core_test

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestSecretMaterialConcurrentCopiesAndDestruction(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name               string
		before, concurrent bool
	}{
		{name: "copies complete before destruction"},
		{name: "destruction completes before copies", before: true},
		{name: "copies race destruction", concurrent: true},
	}
	sizes := []struct {
		name  string
		count int
	}{
		{name: "minimum", count: core.SecretMaterialMinimumBytes},
		{name: "maximum", count: core.SecretMaterialMaximumBytes},
	}
	for _, tc := range cases {
		for _, size := range sizes {
			t.Run(tc.name+"/"+size.name, func(t *testing.T) {
				t.Parallel()
				want := bytes.Repeat([]byte{0x5a}, size.count)
				material, err := core.NewSecretMaterial(want)
				if err != nil {
					t.Fatal(err)
				}
				duration, err := temporal.NewDuration(10 * time.Second)
				if err != nil {
					t.Fatal(err)
				}
				deadline, cancel, err := temporal.WithTimeout(temporal.TimeoutRequest{Parent: t.Context(), Duration: duration})
				if err != nil {
					t.Fatal(err)
				}
				defer cancel()
				if tc.before {
					if err := material.Destroy(); err != nil {
						t.Fatal(err)
					}
				}
				const workers = 8
				type outcome struct {
					data []byte
					err  error
				}
				results := make(chan outcome, workers)
				start := make(chan struct{})
				done := make(chan struct{})
				var group sync.WaitGroup
				group.Add(workers)
				for range workers {
					copiedHandle := material
					go func() {
						defer group.Done()
						select {
						case <-start:
						case <-deadline.Done():
							return
						}
						data, err := copiedHandle.CopyBytes()
						results <- outcome{data: data, err: err}
					}()
				}
				go func() { group.Wait(); close(done) }()
				defer func() {
					cleanup, stop, err := temporal.WithTimeout(temporal.TimeoutRequest{Parent: context.WithoutCancel(t.Context()), Duration: duration})
					if err != nil {
						t.Error(err)
						return
					}
					defer stop()
					cancel()
					select {
					case <-done:
					case <-cleanup.Done():
						t.Errorf("secret-copy cleanup=%v; want all %d workers finished", cleanup.Err(), workers)
					}
				}()
				close(start)
				if tc.concurrent {
					if err := material.Destroy(); err != nil {
						t.Fatal(err)
					}
				}
				for range workers {
					select {
					case got := <-results:
						wantDestroyed := tc.before || tc.concurrent && got.err != nil
						if wantDestroyed {
							if got.data != nil || !errors.Is(got.err, core.ErrPrimitiveContract) {
								t.Fatalf("destroyed copy=%d bytes, %v; want nil and refusal", len(got.data), got.err)
							}
						} else if got.err != nil || !bytes.Equal(got.data, want) {
							t.Fatalf("active copy=%d bytes, %v; want all %d original bytes", len(got.data), got.err, len(want))
						}
					case <-deadline.Done():
						t.Fatalf("copy deadline=%v with %d queued outcomes; want all %d workers observed", deadline.Err(), len(results), workers)
					}
				}
				if err := material.Destroy(); err != nil {
					t.Fatal(err)
				}
				got, err := material.CopyBytes()
				if got != nil || !errors.Is(err, core.ErrPrimitiveContract) {
					t.Fatalf("completed destruction copy=%d bytes, %v; want nil and refusal", len(got), err)
				}
			})
		}
	}
}
