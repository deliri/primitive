package gotoolchain

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gomodule"
	"github.com/deliri/primitive/v2026/process"
	"github.com/deliri/primitive/v2026/runnercontrol"
)

func nativeVetFixture(t *testing.T, directory, source string) (Capability, VetRequest) {
	t.Helper()
	for _, file := range []struct{ name, body string }{{name: "go.mod", body: "module example.com/nativevet\n\ngo 1.27.2\n"}, {name: "subject.go", body: source}} {
		if err := os.WriteFile(filepath.Join(directory, file.name), []byte(file.body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	compiler, observed := installationFixtureCapability(t, directory, nil)
	path, err := gomodule.ParseImportPath("example.com/nativevet")
	if err != nil {
		t.Fatal(err)
	}
	return compiler, VetRequest{WorkingDirectory: observed.WorkingDirectory, Package: path}
}

func TestObserveVetNativeProducerLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, source string
		analyzers    []VetAnalyzer
		lost, wait   int
	}{
		{name: "discarded native cancel", source: "package subject\nimport \"context\"\nfunc F(ctx context.Context){_,_=context.WithCancel(ctx)}", lost: 1},
		{name: "renamed context import remains native", source: "package subject\nimport c \"context\"\nfunc F(ctx c.Context){_,_=c.WithCancel(ctx)}", lost: 1},
		{name: "cancel cause factory remains native", source: "package subject\nimport \"context\"\nfunc F(ctx context.Context){_,_=context.WithCancelCause(ctx)}", lost: 1},
		{name: "discarded native timeout", source: "package subject\nimport(\"context\";\"time\")\nfunc F(ctx context.Context){_,_=context.WithTimeout(ctx,time.Second)}", lost: 1},
		{name: "discarded native deadline", source: "package subject\nimport(\"context\";\"time\")\nfunc F(ctx context.Context,deadline time.Time){_,_=context.WithDeadline(ctx,deadline)}", lost: 1},
		{name: "owned native cancellation is neutral", source: "package subject\nimport \"context\"\nfunc F(ctx context.Context){child,cancel:=context.WithCancel(ctx);defer cancel();_=child}"},
		{name: "returned cancellation transfers ownership", source: "package subject\nimport \"context\"\nfunc F(ctx context.Context)(context.Context,context.CancelFunc){return context.WithCancel(ctx)}"},
		{name: "closure cancellation remains native", source: "package subject\nimport \"context\"\nfunc F(ctx context.Context){func(){_,_=context.WithCancel(ctx)}()}", lost: 1},
		{name: "native WaitGroup with arbitrary variable name", source: "package subject\nimport \"sync\"\nfunc F(counter *sync.WaitGroup){go func(){counter.Add(1);defer counter.Done()}();counter.Wait()}", wait: 1},
		{name: "renamed sync import remains native", source: "package subject\nimport s \"sync\"\nfunc F(counter *s.WaitGroup){go func(){counter.Add(1);defer counter.Done()}();counter.Wait()}", wait: 1},
		{name: "owned native WaitGroup is neutral", source: "package subject\nimport \"sync\"\nfunc F(counter *sync.WaitGroup){counter.Add(1);go func(){defer counter.Done()}();counter.Wait()}"},
		{name: "custom Add receiver cannot borrow sync authority", source: "package subject\ntype group struct{}\nfunc(*group)Add(int){}\nfunc F(wg *group){go func(){wg.Add(1)}()}"},
		{name: "select only lostcancel", source: "package subject\nimport(\"context\";\"sync\")\nfunc F(ctx context.Context,counter *sync.WaitGroup){_,_=context.WithCancel(ctx);go func(){counter.Add(1)}()}", analyzers: []VetAnalyzer{VetAnalyzerLostCancel}, lost: 1},
		{name: "select only waitgroup", source: "package subject\nimport(\"context\";\"sync\")\nfunc F(ctx context.Context,counter *sync.WaitGroup){_,_=context.WithCancel(ctx);go func(){counter.Add(1)}()}", analyzers: []VetAnalyzer{VetAnalyzerWaitGroup}, wait: 1},
		{name: "select both analyzers", source: "package subject\nimport(\"context\";\"sync\")\nfunc F(ctx context.Context,counter *sync.WaitGroup){_,_=context.WithCancel(ctx);go func(){counter.Add(1)}()}", analyzers: []VetAnalyzer{VetAnalyzerLostCancel, VetAnalyzerWaitGroup}, lost: 1, wait: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			compiler, request := nativeVetFixture(t, directory, tc.source)
			request.Analyzers = tc.analyzers
			lost, wait := 0, 0
			request.Visit = func(got runnercontrol.GoVetObservation) error {
				if got.Validate() != nil || got.Kind != runnercontrol.GoVetObservationDiagnostic || got.Unit != "example.com/nativevet" || !strings.HasPrefix(got.Position, filepath.Join(directory, "subject.go")+":") || got.Message == "" {
					t.Fatalf("Native vet=%+v, want positioned producer fact", got)
				}
				switch got.Analyzer {
				case VetAnalyzerLostCancel.String():
					lost++
				case VetAnalyzerWaitGroup.String():
					wait++
				default:
					t.Fatalf("Native analyzer=%q, want requested analyzers", got.Analyzer)
				}
				return nil
			}
			receipt, err := compiler.ObserveVet(t.Context(), request)
			if err != nil || receipt.Validate() != nil || receipt.ExitCode != 0 || lost != tc.lost || wait != tc.wait {
				t.Fatalf("Native vet=(%+v,%v,%d lost,%d wait), want successful process and %d/%d diagnostics", receipt, err, lost, wait, tc.lost, tc.wait)
			}
		})
	}
}

func TestObserveVetNativeRefusals(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		kind   uint8
		want   error
		events int
	}{
		{name: "absent callback refuses before execution", kind: 0, want: core.ErrGoToolchainContract},
		{name: "unknown analyzer refuses before execution", kind: 1, want: core.ErrGoToolchainContract},
		{name: "cancelled owner retains cause", kind: 2, want: context.Canceled},
		{name: "callback refusal retains native cause and zero receipt", kind: 3, want: io.ErrShortWrite, events: 1},
		{name: "callback cancellation retains owner cause and zero receipt", kind: 4, want: context.Canceled, events: 1},
		{name: "native compile refusal is not an empty pass", kind: 5, want: core.ErrGoToolchainExecution},
		{name: "absent native package is not an empty pass", kind: 6, want: core.ErrGoToolchainExecution},
		{name: "absent capability refuses before execution", kind: 7, want: core.ErrGoToolchainContract},
		{name: "absent context refuses before execution", kind: 8, want: core.ErrNilContext},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			source := "package subject\nimport \"context\"\nfunc F(ctx context.Context){_,_=context.WithCancel(ctx)}"
			if tc.kind == 5 {
				source = "package subject\nfunc F(){ undefined() }"
			}
			compiler, request := nativeVetFixture(t, directory, source)
			ctx, cancel := context.WithCancelCause(t.Context())
			defer cancel(nil)
			count := 0
			request.Visit = func(runnercontrol.GoVetObservation) error {
				count++
				if tc.kind == 3 {
					return io.ErrShortWrite
				}
				if tc.kind == 4 {
					cancel(io.ErrClosedPipe)
				}
				return nil
			}
			switch tc.kind {
			case 0:
				request.Visit = nil
			case 1:
				request.Analyzers = []VetAnalyzer{VetAnalyzerUnknown}
			case 2:
				cancel(io.ErrClosedPipe)
			case 6:
				request.Package, _ = gomodule.ParseImportPath("example.com/nativevet/absent")
			case 7:
				compiler = Capability{}
			case 8:
				ctx = nil
			}
			got, err := compiler.ObserveVet(ctx, request)
			if got != (process.ResultObservation{}) || !errors.Is(err, tc.want) || count != tc.events {
				t.Fatalf("Native refusal=(%+v,%v,%d events), want zero,%v,%d", got, err, count, tc.want, tc.events)
			}
			// Contract is the parent identity of native execution/output errors.
			// A child refusal must not impersonate pre-execution validation.
			if (tc.kind == 0 || tc.kind == 1 || tc.kind == 7) && (errors.Is(err, core.ErrGoToolchainExecution) || errors.Is(err, core.ErrGoToolchainOutput)) {
				t.Fatalf("request refusal=%v, want contract validation before native execution", err)
			}
			if (tc.kind == 2 || tc.kind == 4) && !errors.Is(err, io.ErrClosedPipe) {
				t.Fatalf("Native cause=%v, want closed pipe", err)
			}
			if tc.kind == 5 || tc.kind == 6 {
				var exit CommandExitError
				if !errors.As(err, &exit) || exit.Validate() != nil {
					t.Fatalf("Native compile failure=%v, want valid CommandExitError", err)
				}
			}
		})
	}
}

func FuzzObserveVetNativeSourceAuthority(f *testing.F) {
	for mode := uint8(0); mode < 4; mode++ {
		f.Add(mode, uint8(0))
	}
	f.Fuzz(func(t *testing.T, mode, suffix uint8) {
		directory := t.TempDir()
		source := "package subject\nimport \"context\"\nfunc F" + strconv.Itoa(int(suffix)) + "(ctx context.Context){"
		want := 1
		switch mode % 4 {
		case 0:
			source += "_,_=context.WithCancel(ctx)"
		case 1:
			source += "child,cancel:=context.WithCancel(ctx);defer cancel();_=child"
			want = 0
		case 2:
			source += "func(){_,_=context.WithCancel(ctx)}()"
		case 3:
			source += "var child,cancel=context.WithCancel(ctx);defer cancel();_=child"
			want = 0
		}
		source += "}"
		compiler, request := nativeVetFixture(t, directory, source)
		request.Analyzers = []VetAnalyzer{VetAnalyzerLostCancel}
		count := 0
		request.Visit = func(got runnercontrol.GoVetObservation) error {
			count++
			if got.Validate() != nil || got.Analyzer != VetAnalyzerLostCancel.String() || got.Kind != runnercontrol.GoVetObservationDiagnostic || got.Unit != "example.com/nativevet" {
				t.Fatalf("Native observation=%+v, want source-bound cancellation diagnostic", got)
			}
			return nil
		}
		got, err := compiler.ObserveVet(t.Context(), request)
		if err != nil || got.Validate() != nil || got.ExitCode != 0 || count != want {
			t.Fatalf("Native source authority mode%d=(%+v,%v,%d), want valid exit and%d", mode%4, got, err, count, want)
		}
	})
}
