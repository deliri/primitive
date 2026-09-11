package process

import (
	"bytes"
	json "encoding/json/v2"
	"errors"
	"math"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func FuzzPlanJSONExternalIngress(f *testing.F) {
	seed, err := planContractFixture(f.TempDir())
	if err != nil {
		f.Fatal(err)
	}
	for _, tc := range planContractCases() {
		if tc.wantValidate != nil || tc.wantMarshal != nil {
			continue
		}
		p := seed
		p.Arguments = slices.Clone(seed.Arguments)
		p.Environment.Variables = slices.Clone(seed.Environment.Variables)
		if tc.change != nil {
			p, err = tc.change(p)
			if err != nil {
				f.Fatal(err)
			}
		}
		if err := p.Validate(); err != nil {
			f.Fatal(err)
		}
		encoded, err := p.MarshalJSON()
		if err != nil {
			f.Fatal(err)
		}
		f.Add(encoded)
	}
	for _, tc := range planIngressCases() {
		_, encoded, err := planIngressFixture(seed, tc)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(encoded)
	}
	f.Fuzz(func(t *testing.T, input []byte) {
		// Core owns strict JSON syntax. The independent value oracle uses the
		// wire fields directly, not Plan.Validate or its private parser helpers.
		wire, syntaxErr := core.DecodeStrictJSONStructure[planWire](input, core.DefaultStrictJSONLimits())
		wantAccepted := syntaxErr == nil && planWireAdmittedByContract(wire)
		wantCanonical, encodeErr := json.Marshal(wire)
		wantAccepted = wantAccepted && encodeErr == nil && len(wantCanonical) <= core.JSONDocumentMaximumBytes
		receiver := seed
		receiver.Arguments = slices.Clone(seed.Arguments)
		receiver.Environment.Variables = slices.Clone(seed.Environment.Variables)
		before := receiver
		before.Arguments = slices.Clone(receiver.Arguments)
		before.Environment.Variables = slices.Clone(receiver.Environment.Variables)
		inputBefore := bytes.Clone(input)
		err := receiver.UnmarshalJSON(input)
		if !bytes.Equal(input, inputBefore) {
			t.Fatalf("decoder changed caller bytes: got=%x want=%x", input, inputBefore)
		}
		if !wantAccepted {
			if !errors.Is(err, core.ErrProcessContract) || !errors.Is(err, core.ErrJSONContract) || receiver.Command != before.Command || receiver.WorkingDirectory != before.WorkingDirectory || receiver.OutputPolicy != before.OutputPolicy || receiver.WaitDelay != before.WaitDelay || receiver.Containment != before.Containment || receiver.SchemaVersion != before.SchemaVersion || receiver.Environment.Mode != before.Environment.Mode || !slices.Equal(receiver.Arguments, before.Arguments) || !slices.Equal(receiver.Environment.Variables, before.Environment.Variables) || (receiver.Arguments == nil) != (before.Arguments == nil) || (receiver.Environment.Variables == nil) != (before.Environment.Variables == nil) {
				t.Fatalf("refusal changed receiver or lost identity: %v", err)
			}
			return
		}
		if err != nil || receiver.Validate() != nil {
			t.Fatalf("admissible wire refused: %v", err)
		}
		canonical, err := receiver.MarshalJSON()
		if err != nil || !bytes.Equal(canonical, wantCanonical) {
			t.Fatalf("accepted plan changed wire facts: %v", err)
		}
		var next Plan
		if err := next.UnmarshalJSON(canonical); err != nil {
			t.Fatal(err)
		}
		again, err := next.MarshalJSON()
		if err != nil || !bytes.Equal(canonical, again) {
			t.Fatalf("accepted plan lost canonical closure: %v", err)
		}
		bound, err := receiver.Bind(Streams{Stdin: bytes.NewReader(nil), Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}})
		if err != nil || bound.Command != receiver.Command || bound.WorkingDirectory != receiver.WorkingDirectory || bound.Environment.Mode != EnvironmentModeExact || !slices.Equal(bound.Environment.Variables, receiver.Environment.Variables) || !slices.Equal(bound.Arguments, receiver.Arguments) || bound.Containment != receiver.Containment || bound.OutputPolicy != receiver.OutputPolicy || bound.WaitDelay != receiver.WaitDelay {
			t.Fatalf("accepted intent changed at bind: %v", err)
		}
	})
}

func planWireAdmittedByContract(w planWire) bool {
	output, outputErr := w.OutputPolicy.Maximum.Uint64()
	bounded := w.OutputPolicy.Mode == OutputModeBounded && outputErr == nil && output <= math.MaxInt64
	streaming := w.OutputPolicy.Mode == OutputModeStreaming && w.OutputPolicy.Maximum == (core.ByteCount{})
	if w.SchemaVersion != ExecutionPlanSchemaVersion || w.Command.Validate() != nil || w.WorkingDirectory.Validate() != nil || (!bounded && !streaming) || w.WaitDelay.Validate() != nil || w.WaitDelay.IsZero() {
		return false
	}
	if w.Isolation != IsolationDirect.String() && w.Isolation != IsolationGroup.String() {
		return false
	}
	if w.CancelSignal != CancelSignalKill.String() && w.CancelSignal != CancelSignalQuit.String() && w.CancelSignal != CancelSignalInterrupt.String() && w.CancelSignal != CancelSignalTerminate.String() {
		return false
	}
	maximum := int(core.DefaultStrictJSONLimits().ArrayItemMaximum)
	if len(w.Arguments) > maximum || len(w.Environment) > maximum {
		return false
	}
	var argumentBytes uint64
	for _, value := range w.Arguments {
		if strings.IndexByte(value, 0) >= 0 || uint64(len(value)) > ArgumentMaximumBytes {
			return false
		}
		argumentBytes += uint64(len(value)) + 1
	}
	if argumentBytes > ArgumentProjectionMaximumBytes {
		return false
	}
	var environmentBytes uint64
	seen := make(map[string]struct{}, len(w.Environment))
	for _, pair := range w.Environment {
		name, value, found := strings.Cut(pair, "=")
		if !found || name == "" || strings.IndexByte(pair, 0) >= 0 || uint64(len(name)) > EnvironmentNameMaximumBytes || uint64(len(value)) > EnvironmentValueMaximumBytes {
			return false
		}
		environmentBytes += uint64(len(pair)) + 1
		if runtime.GOOS == "windows" {
			name = strings.ToLower(name)
		}
		if _, exists := seen[name]; exists {
			return false
		}
		seen[name] = struct{}{}
	}
	return environmentBytes <= EnvironmentProjectionMaximumBytes
}
