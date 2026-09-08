package process

import (
	"bytes"
	"errors"
	"math"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

type planContractCase struct {
	name         string
	change       func(Plan) (Plan, error)
	wantValidate error
	wantMarshal  error
	wantBytes    int
}

func planContractCases() []planContractCase {
	maximum := int(core.DefaultStrictJSONLimits().ArrayItemMaximum)
	return []planContractCase{
		{name: "positive/exact intent survives every field"},
		{name: "positive/direct quit remains a distinct contract", change: planContainmentChange(IsolationDirect, CancelSignalQuit)},
		{name: "positive/direct interrupt remains a distinct contract", change: planContainmentChange(IsolationDirect, CancelSignalInterrupt)},
		{name: "positive/direct terminate remains a distinct contract", change: planContainmentChange(IsolationDirect, CancelSignalTerminate)},
		{name: "positive/group kill remains a distinct contract", change: planContainmentChange(IsolationGroup, CancelSignalKill)},
		{name: "positive/group quit remains a distinct contract", change: planContainmentChange(IsolationGroup, CancelSignalQuit)},
		{name: "positive/group interrupt remains a distinct contract", change: planContainmentChange(IsolationGroup, CancelSignalInterrupt)},
		{name: "positive/group terminate remains a distinct contract", change: planContainmentChange(IsolationGroup, CancelSignalTerminate)},
		{name: "positive/argument order is exact", change: func(p Plan) (Plan, error) {
			a, e := NewArgument("second")
			p.Arguments = append([]Argument{a}, p.Arguments...)
			return p, e
		}},
		{name: "positive/environment order is exact", change: func(p Plan) (Plan, error) {
			e, err := ParseExactEnvironment([]string{"Z=last", "A=first"})
			p.Environment = e
			return p, err
		}},
		{name: "negative/unknown schema cannot bind", change: func(p Plan) (Plan, error) { p.SchemaVersion = 0; return p, nil }, wantValidate: core.ErrProcessContract, wantMarshal: core.ErrProcessContract},
		{name: "negative/unset command cannot bind", change: func(p Plan) (Plan, error) { p.Command = core.AbsolutePath{}; return p, nil }, wantValidate: core.ErrProcessContract, wantMarshal: core.ErrProcessContract},
		{name: "negative/unset directory cannot bind", change: func(p Plan) (Plan, error) { p.WorkingDirectory = core.AbsolutePath{}; return p, nil }, wantValidate: core.ErrProcessContract, wantMarshal: core.ErrProcessContract},
		{name: "negative/unset argument cannot bind", change: func(p Plan) (Plan, error) { p.Arguments = []Argument{{}}; return p, nil }, wantValidate: core.ErrProcessContract, wantMarshal: core.ErrProcessContract},
		{name: "negative/inheritance is not an exact plan", change: func(p Plan) (Plan, error) { p.Environment = Environment{Mode: EnvironmentModeInherit}; return p, nil }, wantValidate: core.ErrProcessContract, wantMarshal: core.ErrProcessContract},
		{name: "negative/unknown environment mode cannot bind", change: func(p Plan) (Plan, error) { p.Environment.Mode = EnvironmentModeUnknown; return p, nil }, wantValidate: core.ErrProcessContract, wantMarshal: core.ErrProcessContract},
		{name: "negative/duplicate environment cannot bind", change: func(p Plan) (Plan, error) {
			p.Environment.Variables = append(p.Environment.Variables, p.Environment.Variables[0])
			return p, nil
		}, wantValidate: core.ErrProcessContract, wantMarshal: core.ErrProcessContract},
		{name: "negative/unset output limit cannot bind", change: func(p Plan) (Plan, error) { p.OutputLimit = core.ByteCount{}; return p, nil }, wantValidate: core.ErrProcessContract, wantMarshal: core.ErrProcessContract},
		{name: "negative/zero wait delay cannot bind", change: func(p Plan) (Plan, error) { p.WaitDelay = temporal.Duration{}; return p, nil }, wantValidate: core.ErrProcessContract, wantMarshal: core.ErrProcessContract},
		{name: "negative/unknown containment cannot bind", change: planContainmentChange(IsolationUnknown, CancelSignalKill), wantValidate: core.ErrProcessContract, wantMarshal: core.ErrProcessContract},
		{name: "boundary/arguments below JSON array cap", change: planArgumentCountChange(maximum - 1)},
		{name: "boundary/arguments at JSON array cap", change: planArgumentCountChange(maximum)},
		{name: "boundary/arguments above JSON array cap", change: planArgumentCountChange(maximum + 1), wantValidate: core.ErrProcessContract, wantMarshal: core.ErrProcessContract},
		{name: "boundary/environment below JSON array cap", change: planEnvironmentCountChange(maximum - 1)},
		{name: "boundary/environment at JSON array cap", change: planEnvironmentCountChange(maximum)},
		{name: "boundary/environment above JSON array cap", change: planEnvironmentCountChange(maximum + 1), wantValidate: core.ErrProcessContract, wantMarshal: core.ErrProcessContract},
		{name: "boundary/ASCII document below cap", change: planDocumentSizeChange(core.JSONDocumentMaximumBytes-1, "x"), wantBytes: core.JSONDocumentMaximumBytes - 1},
		{name: "boundary/ASCII document at cap", change: planDocumentSizeChange(core.JSONDocumentMaximumBytes, "x"), wantBytes: core.JSONDocumentMaximumBytes},
		{name: "boundary/ASCII document above cap cannot publish", change: planDocumentSizeChange(core.JSONDocumentMaximumBytes+1, "x"), wantMarshal: core.ErrProcessContract},
		{name: "boundary/escaped document below cap", change: planDocumentSizeChange(core.JSONDocumentMaximumBytes-1, "\x01"), wantBytes: core.JSONDocumentMaximumBytes - 1},
		{name: "boundary/escaped document at cap", change: planDocumentSizeChange(core.JSONDocumentMaximumBytes, "\x01"), wantBytes: core.JSONDocumentMaximumBytes},
		{name: "boundary/escaped document above cap cannot publish", change: planDocumentSizeChange(core.JSONDocumentMaximumBytes+1, "\x01"), wantMarshal: core.ErrProcessContract},
		{name: "boundary/minimum output limit survives", change: planOutputChange(1)},
		{name: "boundary/maximum reportable output limit survives", change: planOutputChange(math.MaxInt64)},
		{name: "boundary/unsigned-only output limit cannot bind", change: planOutputChange(uint64(math.MaxInt64) + 1), wantValidate: core.ErrProcessContract, wantMarshal: core.ErrProcessContract},
		{name: "boundary/minimum wait delay survives", change: planWaitChange(1)},
		{name: "boundary/maximum wait delay survives", change: planWaitChange(math.MaxInt64)},
		{name: "boundary/constructed empty argument remains one argument", change: func(p Plan) (Plan, error) { a, e := NewArgument(""); p.Arguments = []Argument{a}; return p, e }},
		{name: "neutral/absent argv remains zero arguments", change: planArgumentCountChange(0)},
		{name: "neutral/empty exact environment remains explicit", change: planEnvironmentCountChange(0)},
	}
}

func TestPlanContractAndPublicationLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range planContractCases() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p, err := planContractFixture(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			if tc.change != nil {
				p, err = tc.change(p)
				if err != nil {
					t.Fatal(err)
				}
			}
			validationErr := p.Validate()
			encoded, marshalErr := p.MarshalJSON()
			if !errors.Is(validationErr, tc.wantValidate) || !errors.Is(marshalErr, tc.wantMarshal) {
				t.Fatalf("plan validation/publication = %v / %v, want %v / %v", validationErr, marshalErr, tc.wantValidate, tc.wantMarshal)
			}
			var output, diagnostic bytes.Buffer
			streams := Streams{Stdin: bytes.NewReader(nil), Stdout: &output, Stderr: &diagnostic}
			bound, bindErr := p.Bind(streams)
			if !errors.Is(bindErr, tc.wantValidate) {
				t.Fatalf("Bind error=%v, want %v", bindErr, tc.wantValidate)
			}
			if output.Len() != 0 || diagnostic.Len() != 0 {
				t.Fatalf("binding wrote stdout=%q stderr=%q; want no output", output.Bytes(), diagnostic.Bytes())
			}
			if tc.wantValidate != nil {
				if bound.Command != (core.AbsolutePath{}) || bound.WorkingDirectory != (core.AbsolutePath{}) || bound.Arguments != nil || bound.Environment.Mode != EnvironmentModeUnknown || bound.Environment.Variables != nil || bound.OutputLimit != (core.ByteCount{}) || bound.WaitDelay != (temporal.Duration{}) || bound.Containment != (Containment{}) || bound.Streams != (Streams{}) {
					t.Fatalf("refusal exposed request: %+v", bound)
				}
			} else {
				if bound.Command != p.Command || bound.WorkingDirectory != p.WorkingDirectory || bound.Containment != p.Containment || bound.OutputLimit != p.OutputLimit || bound.WaitDelay != p.WaitDelay || !slices.Equal(bound.Arguments, p.Arguments) || !slices.Equal(bound.Environment.Variables, p.Environment.Variables) || bound.Environment.Mode != p.Environment.Mode || bound.Streams != streams {
					t.Fatalf("binding changed intent: %+v", bound)
				}
				if len(bound.Arguments) != 0 {
					bound.Arguments[0] = Argument{}
					if p.Arguments[0].Validate() != nil {
						t.Fatalf("binding changed caller argument: got=%+v", p.Arguments[0])
					}
				}
				if len(bound.Environment.Variables) != 0 {
					bound.Environment.Variables[0] = EnvironmentVariable{}
					if p.Environment.Variables[0].Validate() != nil {
						t.Fatalf("binding changed caller environment variable: got=%+v", p.Environment.Variables[0])
					}
				}
			}
			if tc.wantMarshal != nil {
				if encoded != nil || !errors.Is(marshalErr, core.ErrJSONContract) {
					t.Fatalf("failed publication exposed bytes or lost identity: %d, %v", len(encoded), marshalErr)
				}
				return
			}
			if len(encoded) > core.JSONDocumentMaximumBytes || tc.wantBytes != 0 && len(encoded) != tc.wantBytes {
				t.Fatalf("encoded bytes=%d, requested exact extent=%d", len(encoded), tc.wantBytes)
			}
			var decoded Plan
			if err := decoded.UnmarshalJSON(encoded); err != nil {
				t.Fatalf("published plan cannot be consumed: %v", err)
			}
			again, err := decoded.MarshalJSON()
			if err != nil || !bytes.Equal(encoded, again) {
				t.Fatalf("canonical closure failed: %v", err)
			}
			if decoded.Command != p.Command || decoded.WorkingDirectory != p.WorkingDirectory || decoded.Containment != p.Containment || decoded.WaitDelay != p.WaitDelay || decoded.OutputLimit != p.OutputLimit || !slices.Equal(decoded.Arguments, p.Arguments) || !slices.Equal(decoded.Environment.Variables, p.Environment.Variables) || decoded.Environment.Mode != EnvironmentModeExact {
				t.Fatalf("decoded plan lost facts: %+v", decoded)
			}
		})
	}
}

func planContractFixture(root string) (Plan, error) {
	command, a := core.ParseAbsolutePath(filepath.Join(root, "command"))
	directory, b := core.ParseAbsolutePath(root)
	arguments, c := ParseArguments([]string{"first"})
	environment, d := ParseExactEnvironment([]string{"V=value"})
	limit, e := core.NewByteCount(1024)
	wait, f := temporal.DurationFromNanoseconds(17)
	return Plan{Command: command, WorkingDirectory: directory, Arguments: arguments, Environment: environment, OutputLimit: limit, WaitDelay: wait, SchemaVersion: ExecutionPlanSchemaVersion, Containment: Containment{Isolation: IsolationDirect, CancelSignal: CancelSignalKill}}, errors.Join(a, b, c, d, e, f)
}

func planContainmentChange(isolation Isolation, signal CancelSignal) func(Plan) (Plan, error) {
	return func(p Plan) (Plan, error) {
		p.Containment = Containment{Isolation: isolation, CancelSignal: signal}
		return p, nil
	}
}
func planArgumentCountChange(count int) func(Plan) (Plan, error) {
	return func(p Plan) (Plan, error) {
		a, e := NewArgument("arg")
		p.Arguments = make([]Argument, count)
		for i := range p.Arguments {
			p.Arguments[i] = a
		}
		return p, e
	}
}
func planEnvironmentCountChange(count int) func(Plan) (Plan, error) {
	return func(p Plan) (Plan, error) {
		raw := make([]string, count)
		for i := range raw {
			raw[i] = "V" + strconv.Itoa(i) + "=value"
		}
		e, err := ParseExactEnvironment(raw)
		p.Environment = e
		return p, err
	}
}
func planOutputChange(bytes uint64) func(Plan) (Plan, error) {
	return func(p Plan) (Plan, error) { v, e := core.NewByteCount(bytes); p.OutputLimit = v; return p, e }
}
func planWaitChange(nanos int64) func(Plan) (Plan, error) {
	return func(p Plan) (Plan, error) {
		v, e := temporal.DurationFromNanoseconds(nanos)
		p.WaitDelay = v
		return p, e
	}
}
func planDocumentSizeChange(size int, unit string) func(Plan) (Plan, error) {
	return func(p Plan) (Plan, error) {
		empty, e := NewArgument("")
		if e != nil {
			return Plan{}, e
		}
		p.Arguments = []Argument{empty}
		base, e := p.MarshalJSON()
		if e != nil {
			return Plan{}, e
		}
		encodedUnit, e := core.MarshalCanonicalJSONString(unit)
		if e != nil {
			return Plan{}, e
		}
		encodedEmpty, e := core.MarshalCanonicalJSONString("")
		if e != nil {
			return Plan{}, e
		}
		cost := len(encodedUnit) - len(encodedEmpty)
		available := size - len(base)
		payload := strings.Repeat(unit, available/cost) + strings.Repeat("x", available%cost)
		argument, e := NewArgument(payload)
		p.Arguments = []Argument{argument}
		return p, e
	}
}
