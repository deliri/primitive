package process_test

import (
	"bytes"
	"errors"
	"os/exec"
	"slices"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
)

const processFuzzVectorSeparator = byte(0xff)

func FuzzParseArgumentsExternalIngress(f *testing.F) {
	for _, seed := range [][]byte{
		nil,
		{},
		[]byte("one"),
		joinProcessFuzzVector("", "two words", "--flag=value"),
		joinProcessFuzzVector("before", "bad\x00argument", "after"),
		bytes.Repeat([]byte{processFuzzVectorSeparator}, int(process.ArgumentCountMaximum)+1),
		bytes.Repeat([]byte{'a'}, int(process.ArgumentProjectionMaximumBytes)+1),
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		values := processFuzzVector(data)
		wantAccepted := argumentsAdmittedByContract(values)
		got, err := process.ParseArguments(values)
		if !wantAccepted {
			if !errors.Is(err, core.ErrProcessContract) || got != nil {
				t.Fatalf("ParseArguments(%q) = (%v, %v), want nil and %v", values, got, err, core.ErrProcessContract)
			}
			return
		}
		if err != nil || len(got) != len(values) {
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
			requireZeroEnvironmentRefusal(t, got, err)
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
	f.Add(joinProcessFuzzVector("A=old", "B=kept", "A=new", "C=", "B=last"))
	f.Fuzz(func(t *testing.T, data []byte) {
		values := processFuzzVector(data)
		validInput := environmentProjectionsIndividuallyAdmitted(values)
		want := effectiveEnvironmentProjection(values)
		wantAccepted := validInput && exactEnvironmentAdmittedByContract(want)
		got, err := process.ParseEffectiveEnvironment(values)
		if !wantAccepted {
			requireZeroEnvironmentRefusal(t, got, err)
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
	for _, seed := range [][]byte{
		nil,
		{},
		joinProcessFuzzVector("A="),
		joinProcessFuzzVector("A=one", "B=two=three"),
		joinProcessFuzzVector("A=old", "A=new"),
		joinProcessFuzzVector("missing-separator"),
		joinProcessFuzzVector("=missing-name"),
		joinProcessFuzzVector("A=bad\x00value"),
		joinProcessFuzzVector(strings.Repeat("n", int(process.EnvironmentNameMaximumBytes)+1) + "="),
		joinProcessFuzzVector("A=" + strings.Repeat("v", int(process.EnvironmentProjectionMaximumBytes)+1)),
	} {
		f.Add(seed)
	}
}

func joinProcessFuzzVector(values ...string) []byte {
	parts := make([][]byte, len(values))
	for index, value := range values {
		parts[index] = []byte(value)
	}
	return bytes.Join(parts, []byte{processFuzzVectorSeparator})
}

func processFuzzVector(data []byte) []string {
	if data == nil {
		return nil
	}
	parts := bytes.Split(data, []byte{processFuzzVectorSeparator})
	values := make([]string, len(parts))
	for index, part := range parts {
		values[index] = string(part)
	}
	return values
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
	command := exec.Cmd{Env: values}
	if values == nil {
		command.Env = []string{}
	}
	return len(command.Environ()) == len(values)
}

func effectiveEnvironmentProjection(values []string) []string {
	command := exec.Cmd{Env: values}
	if values == nil {
		command.Env = []string{}
	}
	return command.Environ()
}

func requireZeroEnvironmentRefusal(t *testing.T, got process.Environment, err error) {
	t.Helper()
	if !errors.Is(err, core.ErrProcessContract) || got.Mode != process.EnvironmentModeUnknown || got.Variables != nil {
		t.Fatalf("environment refusal = (mode %v, variables %v, error %v), want exact zero and %v", got.Mode, got.Variables, err, core.ErrProcessContract)
	}
}
