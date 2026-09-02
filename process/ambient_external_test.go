package process_test

import (
	"os"
	"testing"

	"github.com/deliri/primitive/v2026/process"
)

func TestAmbientArgumentsPreserveTheCommandOwnedArgvProjection(t *testing.T) {
	t.Parallel()

	got, err := process.AmbientArguments()
	if err != nil {
		t.Fatalf("process.AmbientArguments() error = %v, want nil", err)
	}
	want := os.Args[1:]
	if len(got) != len(want) {
		t.Fatalf("process.AmbientArguments() length = %d, want %d", len(got), len(want))
	}
	for index := range got {
		value, valueErr := got[index].Value()
		if valueErr != nil {
			t.Fatalf("AmbientArguments()[%d].Value() error = %v, want nil", index, valueErr)
		}
		if value != want[index] {
			t.Fatalf("AmbientArguments()[%d] = %q, want %q", index, value, want[index])
		}
	}
}

func TestStandardStreamsExposeTheExactCallingProcessCapabilities(t *testing.T) {
	t.Parallel()

	got, err := process.StandardStreams()
	if err != nil {
		t.Fatalf("process.StandardStreams() error = %v, want nil", err)
	}
	if err := got.Validate(); err != nil {
		t.Fatalf("StandardStreams().Validate() error = %v, want nil", err)
	}
	if got.Stdin != os.Stdin || got.Stdout != os.Stdout || got.Stderr != os.Stderr {
		t.Fatalf("process.StandardStreams() = (%p, %p, %p), want exact standard capabilities (%p, %p, %p)", got.Stdin, got.Stdout, got.Stderr, os.Stdin, os.Stdout, os.Stderr)
	}
}
