package process_test

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
)

// Every public operation has a compiler-bound witness. Raw representation
// doors additionally bind their actual semantic fuzzer below; kernel-owned
// observations and capability constructors are identified separately.
type processExternalDoorInventory struct {
	Alive                      func(process.ProcessIdentity) (process.Liveness, error)
	AmbientArguments           func() ([]process.Argument, error)
	Begin                      func(context.Context, process.Request) (*process.Execution, error)
	DiscardDeviceArgument      func() (process.Argument, error)
	NewArgument                func(string) (process.Argument, error)
	NewEnvironmentName         func(string) (process.EnvironmentName, error)
	NewEnvironmentValue        func(string) (process.EnvironmentValue, error)
	NewTruncatingWriter        func(io.Writer, core.ByteCount) (*process.TruncatingWriter, error)
	ObserveProcesses           func(context.Context, process.ProcessVisit) error
	ParseArguments             func([]string) ([]process.Argument, error)
	ParseEffectiveEnvironment  func([]string) (process.Environment, error)
	ParseExactEnvironment      func([]string) (process.Environment, error)
	Resolve                    func(context.Context, core.PathComponent) (core.AbsolutePath, error)
	ResolveExecutable          func(context.Context, core.AbsolutePath) (core.AbsolutePath, error)
	Run                        func(context.Context, process.Request) (process.Result, error)
	Self                       func() (process.ProcessIdentity, error)
	StandardStreams            func() (process.Streams, error)
	Streams_WriteOutput        func(process.Streams, process.Stream, []byte) (core.ByteLength, error)
	TruncatingWriter_Write     func(*process.TruncatingWriter, []byte) (int, error)
	ResultObservation_Validate func(process.ResultObservation) error
}

var processExternalDoors = processExternalDoorInventory{
	Alive: process.Alive, AmbientArguments: process.AmbientArguments, Begin: process.Begin,
	DiscardDeviceArgument: process.DiscardDeviceArgument,
	NewArgument:           process.NewArgument, NewEnvironmentName: process.NewEnvironmentName, NewEnvironmentValue: process.NewEnvironmentValue,
	NewTruncatingWriter: process.NewTruncatingWriter, ObserveProcesses: process.ObserveProcesses,
	ParseArguments: process.ParseArguments, ParseEffectiveEnvironment: process.ParseEffectiveEnvironment, ParseExactEnvironment: process.ParseExactEnvironment,
	Resolve: process.Resolve, ResolveExecutable: process.ResolveExecutable, Run: process.Run, Self: process.Self, StandardStreams: process.StandardStreams,
	Streams_WriteOutput: process.Streams.WriteOutput, TruncatingWriter_Write: (*process.TruncatingWriter).Write, ResultObservation_Validate: process.ResultObservation.Validate,
}

// Fields deliberately use the same compiler-checked operation names as the
// call inventory. An added operation cannot disappear from classification.
type processExternalFuzzProofInventory struct {
	AmbientArguments, Begin, NewArgument, NewEnvironmentName, NewEnvironmentValue, NewTruncatingWriter,
	ParseArguments, ParseEffectiveEnvironment, ParseExactEnvironment, Resolve, ResolveExecutable, Run,
	Streams_WriteOutput, TruncatingWriter_Write, ResultObservation_Validate func(*testing.F)
}

func processExternalFuzzProofs() processExternalFuzzProofInventory {
	return processExternalFuzzProofInventory{
		AmbientArguments: FuzzParseArgumentsAndAmbientExternalIngress, Begin: FuzzRunAndBeginStreamingExternalIngress,
		NewArgument: FuzzArgumentEnvironmentAtomsExternalIngress, NewEnvironmentName: FuzzArgumentEnvironmentAtomsExternalIngress, NewEnvironmentValue: FuzzArgumentEnvironmentAtomsExternalIngress,
		NewTruncatingWriter: FuzzTruncatingWriterExternalIngress, ParseArguments: FuzzParseArgumentsAndAmbientExternalIngress,
		ParseEffectiveEnvironment: FuzzParseEffectiveEnvironmentExternalIngress, ParseExactEnvironment: FuzzParseExactEnvironmentExternalIngress,
		Resolve: FuzzResolveAndResolveExecutableExternalIngress, ResolveExecutable: FuzzResolveAndResolveExecutableExternalIngress, Run: FuzzRunAndBeginStreamingExternalIngress,
		Streams_WriteOutput: FuzzStreamsWriteOutputSemanticClosure, TruncatingWriter_Write: FuzzTruncatingWriterExternalIngress, ResultObservation_Validate: FuzzResultObservationExternalIngress,
	}
}

// These accept no hostile byte representation. Alive/Self observe the kernel,
// the device/stream constructors return standard OS capabilities, and the
// snapshot's raw row decoder has its own native-leaf fuzz proof. Native tests
// remain necessary: fuzzing a leaf cannot establish a platform API's behavior.
type processNativeMetadataDoorInventory struct {
	Alive                 func(process.ProcessIdentity) (process.Liveness, error)
	DiscardDeviceArgument func() (process.Argument, error)
	ObserveProcesses      func(context.Context, process.ProcessVisit) error
	Self                  func() (process.ProcessIdentity, error)
	StandardStreams       func() (process.Streams, error)
}

func processNativeMetadataDoors() processNativeMetadataDoorInventory {
	return processNativeMetadataDoorInventory{process.Alive, process.DiscardDeviceArgument, process.ObserveProcesses, process.Self, process.StandardStreams}
}

func TestProcessExternalIngressInventoryMatchesProduction(t *testing.T) {
	t.Parallel()

	got, gotErr := scanProcessExternalDoors(".")
	if gotErr != nil {
		t.Fatalf("scanProcessExternalDoors() error = %v, want nil", gotErr)
	}
	want := append(processExternalDoorFieldNames(processExternalDoors), process.ProcessNativeFuzzDoorNames()...)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("Process external doors = %q, want compiler inventory %q", got, want)
	}
	classified := append(processExternalDoorFieldNames(processExternalFuzzProofs()), processExternalDoorFieldNames(processNativeMetadataDoors())...)
	slices.Sort(classified)
	if !slices.Equal(classified, processExternalDoorFieldNames(processExternalDoors)) {
		t.Fatalf("unclassified or duplicate door: %q", classified)
	}

}

func processExternalDoorFieldNames(inventory any) []string {
	typeOf := reflect.TypeOf(inventory)
	fields := make([]string, 0, typeOf.NumField())
	for field := range typeOf.Fields() {
		fields = append(fields, field.Name)
	}
	slices.Sort(fields)
	return fields
}

func scanProcessExternalDoors(root string) ([]string, error) {
	set := token.NewFileSet()
	entries, gotErr := os.ReadDir(root)
	if gotErr != nil {
		return nil, gotErr
	}
	var doors []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") ||
			strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, parseErr := parser.ParseFile(
			set,
			filepath.Join(root, entry.Name()),
			nil,
			parser.SkipObjectResolution,
		)
		if parseErr != nil {
			return nil, parseErr
		}
		doors = append(doors, processParsedExternalDoors(file)...)

	}
	slices.Sort(doors)
	return doors, nil
}

func processParsedExternalDoors(file *ast.File) []string {
	jsonOwners := map[string]bool{}
	for _, decl := range file.Decls {
		general, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range general.Specs {
			typ, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			structure, ok := typ.Type.(*ast.StructType)
			if !ok {
				continue
			}
			for _, field := range structure.Fields.List {
				if field.Tag != nil && strings.Contains(field.Tag.Value, "json:") {
					jsonOwners[typ.Name.Name] = true
				}
			}
		}
	}
	var doors []string
	for _, decl := range file.Decls {
		function, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		name := function.Name.Name
		if function.Recv == nil {
			if ast.IsExported(name) || name == "snapshotSighting" {
				doors = append(doors, name)
			}
			continue
		}
		receiver := function.Recv.List[0].Type
		if pointer, ok := receiver.(*ast.StarExpr); ok {
			receiver = pointer.X
		}
		owner, ok := receiver.(*ast.Ident)
		if !ok || !ast.IsExported(owner.Name) || !ast.IsExported(name) {
			continue
		}
		raw := strings.HasPrefix(name, "Unmarshal") || name == "Validate" && jsonOwners[owner.Name]
		ast.Inspect(function.Type.Params, func(node ast.Node) bool {
			if ident, ok := node.(*ast.Ident); ok && (ident.Name == "byte" || ident.Name == "string") {
				raw = true
			}
			return true
		})
		if raw {
			doors = append(doors, owner.Name+"_"+name)
		}
	}
	slices.Sort(doors)
	return doors
}

func TestProcessIngressScannerLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, source string
		want         []string
	}{
		{name: "positive/new free constructor cannot hide", source: "package p; func Construct(v string){}", want: []string{"Construct"}},
		{name: "positive/pointer decoder cannot hide", source: "package p; type Document struct{}; func (*Document) UnmarshalJSON(v []byte) error{return nil}", want: []string{"Document_UnmarshalJSON"}},
		{name: "positive/value byte effect cannot hide", source: "package p; type Stream struct{}; func (Stream) Write(v []byte){}", want: []string{"Stream_Write"}},
		{name: "positive/JSON fact validation cannot hide", source: "package p; type Fact struct{Value int `json:\"value\"`}; func (Fact) Validate()error{return nil}", want: []string{"Fact_Validate"}},
		{name: "negative/private helper is not a public door", source: "package p; func helper(v string){}"},
		{name: "negative/private writer is not a second public door", source: "package p; type writer struct{}; func (writer) Write(v []byte){}"},
		{name: "neutral/typed projection is not an external decoder", source: "package p; type Fact struct{}; func (Fact) Value()string{return \"\"}"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			file, err := parser.ParseFile(token.NewFileSet(), "fixture.go", tc.source, parser.SkipObjectResolution)
			if err != nil {
				t.Fatal(err)
			}
			got := processParsedExternalDoors(file)
			if !slices.Equal(got, tc.want) {
				t.Fatalf("scanner=%q, want %q", got, tc.want)
			}
		})
	}
}
