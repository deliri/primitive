package hostfacts_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/hostfacts"
	"github.com/deliri/primitive/v2026/process"
	"github.com/deliri/primitive/v2026/testserial"
)

func TestAmbientEnvironmentMutationExecutesExactIntent(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardProcessEnvironment, Scope: core.TestIsolationScopePackageProcess})
	const coordinate = "PRIMITIVE_HOSTFACTS_MUTATION_TEST"
	t.Setenv(coordinate, "before")
	name, err := process.NewEnvironmentName(coordinate)
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"after", "", "  exact\tvalue\n", "value=with=equals"} {
		value, err := process.NewEnvironmentValue(text)
		if err != nil {
			t.Fatal(err)
		}
		if err := hostfacts.SetAmbientEnvironment(t.Context(), process.EnvironmentVariable{Name: name, Value: value}); err != nil {
			t.Fatal(err)
		}
		if got, found := os.LookupEnv(coordinate); !found || got != text {
			t.Fatalf("native environment = (%q,%v), want (%q,true)", got, found, text)
		}
	}
	if err := hostfacts.RemoveAmbientEnvironment(t.Context(), name); err != nil {
		t.Fatal(err)
	}
	if got, found := os.LookupEnv(coordinate); found {
		t.Fatalf("native removed variable = (%q,true), want absence", got)
	}
}

func FuzzAmbientEnvironmentMutationPreservesAdmittedValue(f *testing.F) {
	f.Add("value")
	f.Add("")
	f.Add("\x00")
	f.Add(" value=with\nspacing ")
	f.Fuzz(func(t *testing.T, text string) {
		testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardProcessEnvironment, Scope: core.TestIsolationScopePackageProcess})
		if len(text) > 4096 {
			t.Skip("bounded development corpus")
		}
		const coordinate = "PRIMITIVE_HOSTFACTS_FUZZ_MUTATION_TEST"
		t.Setenv(coordinate, "before")
		name, err := process.NewEnvironmentName(coordinate)
		if err != nil {
			t.Fatal(err)
		}
		value, err := process.NewEnvironmentValue(text)
		if strings.IndexByte(text, 0) >= 0 {
			if err == nil {
				t.Fatal("NUL-bearing value admitted")
			}
			if got := os.Getenv(coordinate); got != "before" {
				t.Fatalf("refused ingress changed environment: %q", got)
			}
			return
		}
		if err != nil {
			t.Fatal(err)
		}
		if err := hostfacts.SetAmbientEnvironment(t.Context(), process.EnvironmentVariable{Name: name, Value: value}); err != nil {
			t.Fatal(err)
		}
		if got, present := os.LookupEnv(coordinate); !present || got != text {
			t.Fatalf("observed value = (%q,%v), want (%q,true)", got, present, text)
		}
		if err := hostfacts.RemoveAmbientEnvironment(t.Context(), name); err != nil {
			t.Fatal(err)
		}
		if _, present := os.LookupEnv(coordinate); present {
			t.Fatal("removed variable still present")
		}
	})
}

func TestAmbientEnvironmentMutationRefusesBeforeExecution(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardProcessEnvironment, Scope: core.TestIsolationScopePackageProcess})
	const coordinate = "PRIMITIVE_HOSTFACTS_REFUSED_MUTATION_TEST"
	t.Setenv(coordinate, "before")
	name, err := process.NewEnvironmentName(coordinate)
	if err != nil {
		t.Fatal(err)
	}
	value, err := process.NewEnvironmentValue("after")
	if err != nil {
		t.Fatal(err)
	}
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	cases := []struct {
		name     string
		ctx      context.Context
		variable process.EnvironmentVariable
		want     error
	}{
		{name: "nil context", variable: process.EnvironmentVariable{Name: name, Value: value}, want: core.ErrNilContext},
		{name: "canceled intent", ctx: canceled, variable: process.EnvironmentVariable{Name: name, Value: value}, want: context.Canceled},
		{name: "missing name", ctx: t.Context(), variable: process.EnvironmentVariable{Value: value}, want: core.ErrHostFactsContract},
		{name: "missing value", ctx: t.Context(), variable: process.EnvironmentVariable{Name: name}, want: core.ErrHostFactsContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(coordinate, "before")
			err := hostfacts.SetAmbientEnvironment(tc.ctx, tc.variable)
			if !errors.Is(err, tc.want) {
				t.Fatalf("set refused = %v, want %v", err, tc.want)
			}
			if got := os.Getenv(coordinate); got != "before" {
				t.Fatalf("refused intent changed native environment to %q", got)
			}
		})
	}
	if err := hostfacts.RemoveAmbientEnvironment(canceled, name); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled removal = %v", err)
	}
	if err := hostfacts.RemoveAmbientEnvironment(t.Context(), process.EnvironmentName{}); !errors.Is(err, core.ErrHostFactsContract) {
		t.Fatalf("invalid removal = %v", err)
	}
	if got := os.Getenv(coordinate); got != "before" {
		t.Fatalf("refused removal changed native environment to %q", got)
	}
}
