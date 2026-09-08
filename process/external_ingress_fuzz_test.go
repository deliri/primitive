package process_test

import (
	"bytes"
	"encoding/gob"
	"errors"
	"io"
	"os"
	"os/exec"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
	"github.com/deliri/primitive/v2026/testserial"
)

// Go's gob encoding preserves every byte and vector boundary in canonical
// corpus fixtures. Arbitrary non-gob mutations reach the constructor as one
// raw element instead of being skipped by a fixture parser.
func FuzzParseArgumentsAndAmbientExternalIngress(f *testing.F) {
	seeds := [][]string{
		nil, {""}, {"one"}, {"", "two words", "--flag=value"},
		{"before", "bad\x00argument", "after"}, {"before\xffafter"},
		make([]string, int(process.ArgumentCountMaximum)+1),
		{strings.Repeat("a", int(process.ArgumentProjectionMaximumBytes)+1)},
	}
	for _, values := range seeds {
		if argumentsAdmittedByContract(values) {
			admitted, err := process.ParseArguments(values)
			if err != nil {
				f.Fatal(err)
			}
			values = make([]string, len(admitted))
			for i, argument := range admitted {
				if err := argument.Validate(); err != nil {
					f.Fatal(err)
				}
				value, err := argument.Value()
				if err != nil {
					f.Fatal(err)
				}
				values[i] = value
			}
		}
		encoded, err := joinProcessFuzzVector(values...)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(encoded)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardProcessArguments, Scope: core.TestIsolationScopePackageProcess})
		values := processFuzzVector(data)
		previous := os.Args
		t.Cleanup(func() { os.Args = previous })
		// Fuzz workers are separate processes and callbacks do not run in
		// parallel. Restore the command-owned argv after each individual input.
		os.Args = append([]string{"owned-fuzz-command"}, values...)
		wantAccepted := argumentsAdmittedByContract(values)
		got, err := process.ParseArguments(values)
		ambient, ambientErr := process.AmbientArguments()
		if !wantAccepted {
			if !errors.Is(err, core.ErrProcessContract) || got != nil || !errors.Is(ambientErr, core.ErrProcessContract) || ambient != nil {
				t.Fatalf("ParseArguments(%q) = (%v, %v), want nil and %v", values, got, err, core.ErrProcessContract)
			}
			return
		}
		if err != nil || ambientErr != nil || !slices.Equal(got, ambient) || len(got) != len(values) {
			t.Fatalf("ParseArguments(%q) = (length %d, %v), want (%d, nil)", values, len(got), err, len(values))
		}
		for index, argument := range got {
			projected, projectionErr := argument.Value()
			if projectionErr != nil || projected != values[index] {
				t.Fatalf("Argument[%d].Value() = (%q, %v), want (%q, nil)", index, projected, projectionErr, values[index])
			}
		}
	})
}

func FuzzParseExactEnvironmentExternalIngress(f *testing.F) {
	addProcessEnvironmentSeeds(f)
	f.Fuzz(func(t *testing.T, data []byte) {
		values := processFuzzVector(data)
		wantAccepted := exactEnvironmentAdmittedByContract(values)
		got, err := process.ParseExactEnvironment(values)
		if !wantAccepted {
			if !errors.Is(err, core.ErrProcessContract) || got.Mode != process.EnvironmentModeUnknown || got.Variables != nil {
				t.Fatalf("environment refusal = (mode %v, variables %v, error %v), want exact zero and %v", got.Mode, got.Variables, err, core.ErrProcessContract)
			}
			return
		}
		projected, projectionErr := got.Strings()
		if err != nil || projectionErr != nil || got.Mode != process.EnvironmentModeExact || !slices.Equal(projected, values) {
			t.Fatalf("ParseExactEnvironment(%q) = (mode %v, projection %q, errors %v), want exact input and nil", values, got.Mode, projected, errors.Join(err, projectionErr))
		}
	})
}

func FuzzParseEffectiveEnvironmentExternalIngress(f *testing.F) {
	addProcessEnvironmentSeeds(f)
	lastWins, err := joinProcessFuzzVector("A=old", "B=kept", "A=new", "C=", "B=last")
	if err != nil {
		f.Fatal(err)
	}
	f.Add(lastWins)
	f.Fuzz(func(t *testing.T, data []byte) {
		values := processFuzzVector(data)
		validInput := environmentProjectionsIndividuallyAdmitted(values)
		want := effectiveEnvironmentProjection(values)
		wantAccepted := validInput && exactEnvironmentAdmittedByContract(want)
		got, err := process.ParseEffectiveEnvironment(values)
		if !wantAccepted {
			if !errors.Is(err, core.ErrProcessContract) || got.Mode != process.EnvironmentModeUnknown || got.Variables != nil {
				t.Fatalf("environment refusal = (mode %v, variables %v, error %v), want exact zero and %v", got.Mode, got.Variables, err, core.ErrProcessContract)
			}
			return
		}
		projected, projectionErr := got.Strings()
		if err != nil || projectionErr != nil || got.Mode != process.EnvironmentModeExact || !slices.Equal(projected, want) {
			t.Fatalf("ParseEffectiveEnvironment(%q) = (mode %v, projection %q, errors %v), want stdlib projection %q and nil", values, got.Mode, projected, errors.Join(err, projectionErr), want)
		}
	})
}

func addProcessEnvironmentSeeds(f *testing.F) {
	f.Helper()
	for _, seed := range exactEnvironmentAdmissionCases() {
		values := seed.values
		if seed.wantErr == nil {
			admitted, err := process.ParseExactEnvironment(values)
			if err != nil {
				f.Fatal(err)
			}
			if err := admitted.Validate(); err != nil {
				f.Fatal(err)
			}
			values, err = admitted.Strings()
			if err != nil {
				f.Fatal(err)
			}
		}
		encoded, err := joinProcessFuzzVector(values...)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(encoded)
	}
}

func joinProcessFuzzVector(values ...string) ([]byte, error) {
	var encoded bytes.Buffer
	err := gob.NewEncoder(&encoded).Encode(values)
	return encoded.Bytes(), err
}

func processFuzzVector(data []byte) []string {
	var values []string
	decoder := gob.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&values); err == nil {
		var trailing []string
		if errors.Is(decoder.Decode(&trailing), io.EOF) {
			return values
		}
	}
	return []string{string(data)}
}

func argumentsAdmittedByContract(values []string) bool {
	if uint64(len(values)) > uint64(process.ArgumentCountMaximum) {
		return false
	}
	var projection uint64
	for _, value := range values {
		if strings.IndexByte(value, 0) >= 0 || uint64(len(value)) > process.ArgumentMaximumBytes ||
			uint64(len(value)) >= process.ArgumentProjectionMaximumBytes ||
			projection > process.ArgumentProjectionMaximumBytes-(uint64(len(value))+1) {
			return false
		}
		projection += uint64(len(value)) + 1
	}
	return true
}

func environmentProjectionsIndividuallyAdmitted(values []string) bool {
	if uint64(len(values)) > uint64(process.EnvironmentVariableCountMaximum) {
		return false
	}
	var projection uint64
	for _, value := range values {
		name, content, found := strings.Cut(value, "=")
		if !found || name == "" || strings.ContainsAny(name, "=\x00") || strings.IndexByte(content, 0) >= 0 ||
			uint64(len(name)) > process.EnvironmentNameMaximumBytes || uint64(len(content)) > process.EnvironmentValueMaximumBytes ||
			uint64(len(value)) >= process.EnvironmentProjectionMaximumBytes ||
			projection > process.EnvironmentProjectionMaximumBytes-(uint64(len(value))+1) {
			return false
		}
		projection += uint64(len(value)) + 1
	}
	return true
}

func exactEnvironmentAdmittedByContract(values []string) bool {
	if !environmentProjectionsIndividuallyAdmitted(values) {
		return false
	}
	// Count identity directly, independently of production's comparison with
	// Cmd.Environ. Go dedupEnvCase uses strings.ToLower on Windows; critical
	// variables that Go appends are not duplicates of the caller's input.
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		name, _, _ := strings.Cut(value, "=")
		if runtime.GOOS == "windows" {
			name = strings.ToLower(name)
		}
		if _, duplicate := seen[name]; duplicate {
			return false
		}
		seen[name] = struct{}{}
	}
	return true
}

func effectiveEnvironmentProjection(values []string) []string {
	command := exec.Cmd{Env: values}
	if values == nil {
		command.Env = []string{}
	}
	return command.Environ()
}
