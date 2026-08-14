package hostfacts

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// TestClassifyRotationalFlagAdmitsOnlyTheDocumentedInterface exhausts the
// complete valid space of the kernel's rotational declaration, one token
// zero or one with one optional trailing newline, and rejects every way a
// source can stop speaking the documented interface.
func TestClassifyRotationalFlagAdmitsOnlyTheDocumentedInterface(t *testing.T) {
	t.Parallel()

	cases := []struct {
		wantErr error
		name    string
		input   string
		want    DiskRotation
	}{
		{name: "bare zero is non-rotational", input: "0", want: DiskRotationNonRotational},
		{name: "bare one is rotational", input: "1", want: DiskRotationRotational},
		{name: "zero with the trailing newline is non-rotational", input: "0\n", want: DiskRotationNonRotational},
		{name: "one with the trailing newline is rotational", input: "1\n", want: DiskRotationRotational},
		{name: "empty content is refused", input: "", wantErr: core.ErrHostFactsObservation},
		{name: "a lone newline is refused", input: "\n", wantErr: core.ErrHostFactsObservation},
		{name: "two newlines carry no token", input: "\n\n", wantErr: core.ErrHostFactsObservation},
		{name: "a second trailing newline is refused", input: "0\n\n", wantErr: core.ErrHostFactsObservation},
		{name: "the digit above the domain is refused", input: "2", wantErr: core.ErrHostFactsObservation},
		{name: "a leading zero respelling is refused", input: "01", wantErr: core.ErrHostFactsObservation},
		{name: "a trailing zero respelling is refused", input: "10", wantErr: core.ErrHostFactsObservation},
		{name: "a doubled zero is refused", input: "00", wantErr: core.ErrHostFactsObservation},
		{name: "a doubled one is refused", input: "11", wantErr: core.ErrHostFactsObservation},
		{name: "a negative flag is refused", input: "-1", wantErr: core.ErrHostFactsObservation},
		{name: "an explicitly positive flag is refused", input: "+1", wantErr: core.ErrHostFactsObservation},
		{name: "a leading space is refused", input: " 0", wantErr: core.ErrHostFactsObservation},
		{name: "a trailing space is refused", input: "0 ", wantErr: core.ErrHostFactsObservation},
		{name: "an interior tab is refused", input: "0\t1", wantErr: core.ErrHostFactsObservation},
		{name: "a carriage return line ending is refused", input: "0\r\n", wantErr: core.ErrHostFactsObservation},
		{name: "a bare carriage return is refused", input: "1\r", wantErr: core.ErrHostFactsObservation},
		{name: "two tokens on two lines are refused", input: "0\n1", wantErr: core.ErrHostFactsObservation},
		{name: "an embedded NUL is refused", input: "0\x00", wantErr: core.ErrHostFactsObservation},
		{name: "a prose answer is refused", input: "true", wantErr: core.ErrHostFactsObservation},
		{name: "a vendor spelling is refused", input: "ssd", wantErr: core.ErrHostFactsObservation},
		{name: "a hexadecimal respelling is refused", input: "0x1", wantErr: core.ErrHostFactsObservation},
		{name: "a non-ascii digit is refused", input: "١", wantErr: core.ErrHostFactsObservation},
		{name: "a full-extent token outside the domain is refused", input: "0000000000000000", wantErr: core.ErrHostFactsObservation},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := classifyRotationalFlag([]byte(tc.input))
			if tc.wantErr != nil {
				if !errors.Is(gotErr, tc.wantErr) {
					t.Fatalf("classifyRotationalFlag(%q) error = %v, want errors.Is(..., %v)", tc.input, gotErr, core.ErrHostFactsObservation)
				}
				if got != DiskRotationUnknown {
					t.Fatalf("classifyRotationalFlag(%q) = %v, want %v on rejection", tc.input, got, DiskRotationUnknown)
				}
				return
			}
			if gotErr != nil {
				t.Fatalf("classifyRotationalFlag(%q) error = %v, want nil", tc.input, gotErr)
			}
			if got != tc.want {
				t.Fatalf("classifyRotationalFlag(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

// TestObserveDiskRotationHoldsItsContractGates pins the door's refusals on
// every platform: a dead context, an unset request, and a directory nothing
// occupies are all refused before any answer is invented, and every refusal
// carries the zero rotation.
func TestObserveDiskRotationHoldsItsContractGates(t *testing.T) {
	t.Parallel()

	t.Run("cancelled context is refused before the filesystem is touched", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		directory, err := core.ParseAbsolutePath(dir)
		if err != nil {
			t.Fatalf("ParseAbsolutePath(%q) error = %v, want nil", dir, err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		got, gotErr := ObserveDiskRotation(ctx, DiskRotationRequest{Directory: directory})
		if !errors.Is(gotErr, context.Canceled) || got != DiskRotationUnknown {
			t.Fatalf("ObserveDiskRotation(cancelled) = (%v, %v), want (%v, errors.Is context.Canceled)",
				got, gotErr, DiskRotationUnknown)
		}
	})

	t.Run("unset request is a contract violation", func(t *testing.T) {
		t.Parallel()
		got, gotErr := ObserveDiskRotation(context.Background(), DiskRotationRequest{})
		if !errors.Is(gotErr, core.ErrHostFactsContract) || got != DiskRotationUnknown {
			t.Fatalf("ObserveDiskRotation(zero request) = (%v, %v), want (%v, errors.Is %v)",
				got, gotErr, DiskRotationUnknown, core.ErrHostFactsContract)
		}
	})

	t.Run("directory nothing occupies is a failed observation with the native cause", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		absent, err := core.ParseAbsolutePath(filepath.Join(dir, "absent"))
		if err != nil {
			t.Fatalf("ParseAbsolutePath() error = %v, want nil", err)
		}
		got, gotErr := ObserveDiskRotation(context.Background(), DiskRotationRequest{Directory: absent})
		if !errors.Is(gotErr, core.ErrHostFacts) || !errors.Is(gotErr, fs.ErrNotExist) || got != DiskRotationUnknown {
			t.Fatalf("ObserveDiskRotation(absent) = (%v, %v), want (%v, hostfacts identity with fs.ErrNotExist reachable)",
				got, gotErr, DiskRotationUnknown)
		}
	})
}
