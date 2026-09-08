package hostfacts_test

import (
	"context"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/hostfacts"
)

func FuzzResolveWorkingPathSemanticClosure(f *testing.F) {
	working, err := hostfacts.WorkingDirectory()
	if err != nil {
		f.Fatalf("working-directory seed = %v, want nil", err)
	}
	f.Add(working.String())
	f.Add(".")
	f.Add("../child")
	f.Add("")
	f.Add("child\x00tail")
	f.Fuzz(func(t *testing.T, text string) {
		got, err := hostfacts.ResolveWorkingPath(t.Context(), text)
		want, wantErr := working.ResolveText(text)
		if wantErr != nil {
			if got != (core.AbsolutePath{}) || !errors.Is(err, core.ErrHostFactsContract) || errors.Is(err, core.ErrHostFactsObservation) || !errors.Is(err, core.ErrPrimitiveContract) {
				t.Fatalf("path refusal = %v/%v, want zero with owning error identities", got, err)
			}
			return
		}
		if err != nil || got != want || got.Validate() != nil {
			t.Fatalf("resolved path = %v/%v, want exact core resolution %v", got, err, want)
		}
	})
}

var _ = struct {
	Door func(context.Context, string) (core.AbsolutePath, error)
	Fuzz func(*testing.F)
}{Door: hostfacts.ResolveWorkingPath, Fuzz: FuzzResolveWorkingPathSemanticClosure}
