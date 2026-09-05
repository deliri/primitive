package capabilities

import (
	"context"
	"errors"
	"reflect"
	"runtime"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/gomodule"
	"github.com/deliri/primitive/v2026/process"
	"github.com/deliri/primitive/v2026/temporal"
)

// A removed operation or changed request/result contract breaks compilation.
var (
	_ func(context.Context, filestore.ReadRequest) (core.ByteLength, error)          = filestore.Read
	_ func(context.Context, filestore.WriteRequest) (filestore.CommitRequest, error) = filestore.Write
	_ func(context.Context, process.Request) (process.Result, error)                 = process.Run
	_ func() (temporal.Observation, error)                                           = temporal.Observe
)

func TestCallableOperationsLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		operation Operation
		function  reflect.Value
	}{
		{"bounded file read", OperationReadFile, reflect.ValueOf(filestore.Read)},
		{"durable file write", OperationWriteFile, reflect.ValueOf(filestore.Write)},
		{"bounded process execution", OperationRunProcess, reflect.ValueOf(process.Run)},
		{"one exact clock observation", OperationObserveTime, reflect.ValueOf(temporal.Observe)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, found, err := tc.operation.Contract()
			if err != nil || !found {
				t.Fatalf("operation contract = (%+v,%t,%v), want offered", got, found, err)
			}
			wantName := runtime.FuncForPC(tc.function.Pointer()).Name()
			gotName := got.Function.ImportPath.String() + "." + got.Function.Selector.String()
			if gotName != wantName {
				t.Fatalf("operation symbol = %s, want compiler symbol %s", gotName, wantName)
			}
			signature := tc.function.Type()
			resultPath, err := got.ResultPackage.ImportPath()
			if err != nil {
				t.Fatal(err)
			}
			if got.Result.String() != signature.Out(0).Name() || resultPath != signature.Out(0).PkgPath() {
				t.Fatalf("result contract = %s.%s, want compiler type %s", resultPath, got.Result, signature.Out(0))
			}
			if signature.NumIn() == 0 {
				if got.HasRequest {
					t.Fatalf("request present = true for %s, want false", wantName)
				}
				return
			}
			request := signature.In(signature.NumIn() - 1)
			if !got.HasRequest || got.Request.String() != request.Name() || got.Function.ImportPath.String() != request.PkgPath() {
				t.Fatalf("request contract = %v/%s, want compiler type %s", got.HasRequest, got.Request, request)
			}
		})
	}
	for raw := 0; raw <= 255; raw++ {
		operation := Operation(raw)
		contract, found, err := operation.Contract()
		if raw >= int(operationLimit) {
			if !errors.Is(err, core.ErrCapabilitiesContract) || found || contract != (OperationContract{}) {
				t.Fatalf("unknown operation %d = (%+v,%t,%v), want zero typed refusal", raw, contract, found, err)
			}
			continue
		}
		if raw == int(OperationUnavailable) && (found || err != nil || contract != (OperationContract{})) {
			t.Fatalf("unavailable operation = (%+v,%t,%v), want empty knowledge", contract, found, err)
		}
	}
	path, err := gomodule.ParseImportPath("os")
	if err != nil {
		t.Fatal(err)
	}
	selector, err := ParseSymbolName("Exit")
	if err != nil {
		t.Fatal(err)
	}
	fact, err := ResolveStandardSymbol(StandardSymbol{ImportPath: path, Selector: selector})
	if err != nil || fact.Effect != EffectProcess || fact.Operation != OperationUnavailable {
		t.Fatalf("os.Exit = (%+v,%v), want process ownership with unavailable replacement", fact, err)
	}
}

func FuzzOperationJSONSemanticClosure(f *testing.F) {
	for operation := range operationLimit {
		data, err := operation.MarshalJSON()
		if err != nil {
			f.Fatal(err)
		}
		f.Add(data)
	}
	f.Add([]byte{})
	f.Add([]byte(`"future"`))
	f.Fuzz(func(t *testing.T, data []byte) {
		got := OperationReadFile
		err := got.UnmarshalJSON(data)
		if err != nil {
			if !errors.Is(err, core.ErrCapabilitiesContract) || got != OperationReadFile {
				t.Fatalf("operation decode = (%v,%v), want unchanged and typed refusal", got, err)
			}
			return
		}
		if err := got.Validate(); err != nil {
			t.Fatal(err)
		}
		contract, found, err := got.Contract()
		if err != nil {
			t.Fatal(err)
		}
		if found {
			if err := contract.Validate(); err != nil {
				t.Fatal(err)
			}
		}
		canonical, err := got.MarshalJSON()
		if err != nil {
			t.Fatal(err)
		}
		var second Operation
		if err := second.UnmarshalJSON(canonical); err != nil || second != got {
			t.Fatalf("operation round trip = (%v,%v), want %v", second, err, got)
		}
	})
}
