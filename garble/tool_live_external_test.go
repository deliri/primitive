package garble_test

import (
	"bytes"
	"context"
	"debug/buildinfo"
	"encoding/base64"
	"errors"
	"io"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/garble"
)

const (
	pinnedToolProbeTimeout            = 10 * time.Second
	pinnedToolProbeOutputMaximumBytes = 32 << 10
	pinnedToolVersionCommand          = "version"
)

type boundedProbeOutput struct {
	value   []byte
	maximum int
}

func (o *boundedProbeOutput) Write(value []byte) (int, error) {
	remaining := o.maximum - len(o.value)
	if remaining <= 0 {
		return 0, io.ErrShortWrite
	}
	if len(value) > remaining {
		o.value = append(o.value, value[:remaining]...)
		return remaining, io.ErrShortWrite
	}
	o.value = append(o.value, value...)
	return len(value), nil
}

func TestPinnedGarbleToolProvesWhyPrimitiveRejectsLossySeedForms(t *testing.T) {
	t.Parallel()

	path, gotPathErr := exec.LookPath("garble")
	if gotPathErr != nil {
		t.Fatalf("exec.LookPath(garble) error = %v, want pinned tool installed", gotPathErr)
	}
	canonical := base64.RawStdEncoding.EncodeToString(make([]byte, garble.SeedBytes))
	sevenBytes := base64.RawStdEncoding.EncodeToString(make([]byte, garble.SeedBytes-1))
	nineBytes := base64.RawStdEncoding.EncodeToString(make([]byte, garble.SeedBytes+1))
	cases := []struct {
		name             string
		seed             string
		wantPrimitiveErr bool
		wantExitErr      bool
		wantStderr       bool
	}{
		{name: "exact eight bytes are accepted silently", seed: canonical},
		{name: "padding is normalized silently upstream but rejected by Primitive", seed: canonical + "=", wantPrimitiveErr: true},
		{name: "nine bytes are truncated with a terminal warning upstream", seed: nineBytes, wantPrimitiveErr: true, wantStderr: true},
		{name: "seven bytes are rejected by both boundaries", seed: sevenBytes, wantPrimitiveErr: true, wantExitErr: true, wantStderr: true},
		{name: "invalid base64 is rejected by both boundaries", seed: "not_base64!", wantPrimitiveErr: true, wantExitErr: true, wantStderr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, gotPrimitiveErr := garble.ParseSeed(tc.seed)
			if tc.wantPrimitiveErr {
				if !errors.Is(gotPrimitiveErr, core.ErrGarbleContract) ||
					!errors.Is(gotPrimitiveErr, core.ErrPrimitiveContract) {
					t.Fatalf("ParseSeed() error = %v, want %v and %v", gotPrimitiveErr, core.ErrGarbleContract, core.ErrPrimitiveContract)
				}
			} else if gotPrimitiveErr != nil {
				t.Fatalf("ParseSeed() error = %v, want nil", gotPrimitiveErr)
			}

			ctx, cancel := context.WithTimeout(context.Background(), pinnedToolProbeTimeout)
			defer cancel()
			command := exec.CommandContext(ctx, path, "-seed="+tc.seed, pinnedToolVersionCommand)
			var stdout boundedProbeOutput
			stdout.maximum = pinnedToolProbeOutputMaximumBytes
			var stderr boundedProbeOutput
			stderr.maximum = pinnedToolProbeOutputMaximumBytes
			command.Stdout = &stdout
			command.Stderr = &stderr
			gotRunErr := command.Run()
			var gotExitErr *exec.ExitError
			if tc.wantExitErr {
				if !errors.As(gotRunErr, &gotExitErr) {
					t.Fatalf("pinned Garble run error = %v, want *exec.ExitError", gotRunErr)
				}
			} else if gotRunErr != nil {
				t.Fatalf("pinned Garble run error = %v, want nil", gotRunErr)
			}
			if gotStderr := len(stderr.value) != 0; gotStderr != tc.wantStderr {
				t.Fatalf("pinned Garble stderr present = %t (%q), want %t", gotStderr, stderr.value, tc.wantStderr)
			}
		})
	}
}

func TestPinnedGarbleToolAcceptsEveryTypedPolicyWithoutComplaint(t *testing.T) {
	t.Parallel()

	path, gotPathErr := exec.LookPath("garble")
	if gotPathErr != nil {
		t.Fatalf("exec.LookPath(garble) error = %v, want pinned tool installed", gotPathErr)
	}
	seed := garble.NewSeed([garble.SeedBytes]byte{})
	cases := []struct {
		name        string
		literals    garble.LiteralPolicy
		diagnostics garble.DiagnosticPolicy
	}{
		{name: "canonical seed only is silent", literals: garble.LiteralPolicyPreserve, diagnostics: garble.DiagnosticPolicyPreserve},
		{name: "literal obfuscation is silent", literals: garble.LiteralPolicyObfuscate, diagnostics: garble.DiagnosticPolicyPreserve},
		{name: "diagnostic stripping is silent", literals: garble.LiteralPolicyPreserve, diagnostics: garble.DiagnosticPolicyStrip},
		{name: "both documented opt-ins are silent", literals: garble.LiteralPolicyObfuscate, diagnostics: garble.DiagnosticPolicyStrip},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			intent, gotPrepareErr := garble.PrepareBuild(garble.BuildRequest{
				Tool:        garble.CurrentTool(),
				Seed:        seed,
				Literals:    tc.literals,
				Diagnostics: tc.diagnostics,
			})
			if gotPrepareErr != nil {
				t.Fatalf("PrepareBuild() error = %v, want nil", gotPrepareErr)
			}
			sequence, gotArgumentsErr := intent.Arguments()
			if gotArgumentsErr != nil {
				t.Fatalf("BuildIntent.Arguments() error = %v, want nil", gotArgumentsErr)
			}
			var arguments []string
			for argument := range sequence {
				kind, gotKindErr := argument.Kind()
				if gotKindErr != nil {
					t.Fatalf("Argument.Kind() error = %v, want nil", gotKindErr)
				}
				if kind == garble.ArgumentKindBuild {
					continue
				}
				text, gotTextErr := argument.Text()
				if gotTextErr != nil {
					t.Fatalf("Argument.Text() error = %v, want nil", gotTextErr)
				}
				arguments = append(arguments, text)
			}
			arguments = append(arguments, pinnedToolVersionCommand)

			ctx, cancel := context.WithTimeout(context.Background(), pinnedToolProbeTimeout)
			defer cancel()
			command := exec.CommandContext(ctx, path, arguments...)
			var stdout boundedProbeOutput
			stdout.maximum = pinnedToolProbeOutputMaximumBytes
			var stderr boundedProbeOutput
			stderr.maximum = pinnedToolProbeOutputMaximumBytes
			command.Stdout = &stdout
			command.Stderr = &stderr
			gotRunErr := command.Run()
			if gotRunErr != nil {
				t.Fatalf("pinned Garble typed-argument probe error = %v, want nil", gotRunErr)
			}
			if len(stderr.value) != 0 {
				t.Fatalf("pinned Garble typed-argument stderr bytes = %d, want 0: %q", len(stderr.value), stderr.value)
			}
			if len(stdout.value) == 0 {
				t.Fatal("pinned Garble typed-argument stdout bytes = 0, want version output")
			}
		})
	}
}

func TestPinnedGarbleBinaryMatchesCompilerOwnedIdentity(t *testing.T) {
	t.Parallel()

	path, gotPathErr := exec.LookPath("garble")
	if gotPathErr != nil {
		t.Fatalf("exec.LookPath(garble) error = %v, want pinned tool installed", gotPathErr)
	}
	tool := garble.CurrentTool()
	module, gotModuleErr := tool.ModulePath()
	version, gotVersionErr := tool.Version()
	moduleSum, gotModuleSumErr := tool.ModuleSum()
	if gotModuleErr != nil || gotVersionErr != nil || gotModuleSumErr != nil {
		t.Fatalf("tool identity projection errors = (%v, %v, %v), want all nil", gotModuleErr, gotVersionErr, gotModuleSumErr)
	}

	info, gotInfoErr := buildinfo.ReadFile(path)
	if gotInfoErr != nil {
		t.Fatalf("buildinfo.ReadFile(%q) error = %v, want nil", path, gotInfoErr)
	}
	if info.Main.Path != module ||
		info.Main.Version != version ||
		info.Main.Sum != moduleSum {
		t.Fatalf(
			"pinned Garble build identity = (%q, %q, %q), want (%q, %q, %q)",
			info.Main.Path,
			info.Main.Version,
			info.Main.Sum,
			module,
			version,
			moduleSum,
		)
	}

	ctx, cancel := context.WithTimeout(context.Background(), pinnedToolProbeTimeout)
	defer cancel()
	command := exec.CommandContext(ctx, path, pinnedToolVersionCommand)
	var stdout boundedProbeOutput
	stdout.maximum = pinnedToolProbeOutputMaximumBytes
	var stderr boundedProbeOutput
	stderr.maximum = pinnedToolProbeOutputMaximumBytes
	command.Stdout = &stdout
	command.Stderr = &stderr
	gotRunErr := command.Run()
	if gotRunErr != nil {
		t.Fatalf("pinned Garble version probe error = %v, want nil", gotRunErr)
	}
	if len(stderr.value) != 0 {
		t.Fatalf("pinned Garble version stderr bytes = %d, want 0: %q", len(stderr.value), stderr.value)
	}
	firstLine, _, _ := strings.Cut(string(bytes.TrimSpace(stdout.value)), "\n")
	fields := strings.Fields(firstLine)
	if len(fields) != 2 || fields[0] != module || fields[1] != version {
		t.Fatalf("pinned Garble version output = %q, want module %q version %q", firstLine, module, version)
	}
}
