package filestore

import (
	"fmt"
	"go/parser"
	"go/token"
	"io"
	"reflect"
	"testing"
)

func TestStreamAdapterArchitectureExceptionsStayExact(t *testing.T) {
	t.Parallel()
	reader := reflect.TypeFor[streamReader]().Name()
	writer := reflect.TypeFor[streamWriter]().Name()
	held := reflect.TypeFor[HeldDirectory]().Name()
	read := reflect.TypeFor[io.Reader]().Method(0).Name
	write := reflect.TypeFor[io.Writer]().Method(0).Name
	closeMethod := reflect.TypeFor[io.Closer]().Method(0).Name
	for _, tc := range []struct {
		name           string
		receiver       string
		method         string
		wantViolations int
	}{
		{name: "validated reader retains Go reader method", receiver: reader, method: read},
		{name: "validated writer retains Go writer method", receiver: writer, method: write},
		{name: "owned directory retains Go close method", receiver: held, method: closeMethod},
		{name: "reader cannot acquire writer capability", receiver: reader, method: write, wantViolations: 1},
		{name: "reader cannot close caller custody", receiver: reader, method: closeMethod, wantViolations: 1},
		{name: "writer cannot acquire reader capability", receiver: writer, method: read, wantViolations: 1},
		{name: "writer cannot close caller custody", receiver: writer, method: closeMethod, wantViolations: 1},
		{name: "held directory cannot become byte reader", receiver: held, method: read, wantViolations: 1},
		{name: "held directory cannot become byte writer", receiver: held, method: write, wantViolations: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// Parsing is intentional: each synthetic declaration must exercise
			// the real scanner rather than only its exception predicate.
			source := fmt.Sprintf("package filestore\nfunc (*%s) %s() {}", tc.receiver, tc.method)
			file, err := parser.ParseFile(token.NewFileSet(), "synthetic_adapter.go", source, parser.SkipObjectResolution)
			if err != nil {
				t.Fatal(err)
			}
			got, err := scanProductionArchitecture([]productionFile{{file: file, name: "synthetic_adapter.go"}})
			if err != nil {
				t.Fatal(err)
			}
			if len(got.violations) != tc.wantViolations {
				t.Fatalf("violations = %v, want %d", got.violations, tc.wantViolations)
			}
		})
	}
}
