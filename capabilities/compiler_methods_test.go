package capabilities

import (
	"go/importer"
	"go/types"
	"testing"

	"github.com/deliri/primitive/v2026/gomodule"
)

// These are the receiver types in the declared Go-toolchain coverage scope.
// Every actual exported method, including a promoted method, must classify.
// A Go upgrade adding an unclassified method makes this test fail.
func TestCompilerExportedMethodSetsHaveCatalogCoverage(t *testing.T) {
	t.Parallel()
	cases := []struct {
		imported  string
		receivers []string
	}{
		{"os", []string{"File", "Root", "Process"}},
		{"os/exec", []string{"Cmd"}},
		{"net", []string{"Conn", "PacketConn", "Listener", "TCPConn", "UDPConn", "UnixConn", "IPConn", "Dialer", "ListenConfig"}},
		{"net/http", []string{"Client", "Header", "ResponseWriter", "ResponseController", "Flusher", "Hijacker", "RoundTripper", "Server", "Transport"}},
		{"time", []string{"Timer", "Ticker"}},
		{"syscall", []string{"RawConn"}},
	}
	for _, tc := range cases {
		t.Run(tc.imported+" exported receiver coverage", func(t *testing.T) {
			t.Parallel()
			pkg, err := importer.Default().Import(tc.imported)
			if err != nil {
				t.Fatalf("compiler import %s error = %v, want nil", tc.imported, err)
			}
			for _, name := range tc.receivers {
				object := pkg.Scope().Lookup(name)
				if object == nil {
					t.Fatalf("compiler type %s.%s = nil, want present", tc.imported, name)
				}
				typ := object.Type()
				if _, ok := typ.Underlying().(*types.Interface); !ok {
					typ = types.NewPointer(typ)
				}
				methods := types.NewMethodSet(typ)
				if methods.Len() == 0 {
					t.Fatalf("%s.%s method count = 0, want non-vacuous coverage", tc.imported, name)
				}
				for method := range methods.Methods() {
					function, ok := method.Obj().(*types.Func)
					if !ok || !function.Exported() {
						continue
					}
					signature := function.Type().(*types.Signature)
					receiverType := types.Unalias(signature.Recv().Type())
					if pointer, ok := receiverType.(*types.Pointer); ok {
						receiverType = types.Unalias(pointer.Elem())
					}
					receiver, err := ParseSymbolName(receiverType.(*types.Named).Obj().Name())
					if err != nil {
						t.Fatal(err)
					}
					selector, err := ParseSymbolName(function.Name())
					if err != nil {
						t.Fatal(err)
					}
					imported, err := gomodule.ParseImportPath(function.Pkg().Path())
					if err != nil {
						t.Fatal(err)
					}
					fact, err := ResolveStandardSymbol(StandardSymbol{ImportPath: imported, Receiver: &receiver, Selector: selector})
					if err != nil || fact.Disposition == StandardSymbolUnresolved {
						t.Errorf("%s.%s.%s declares %s.%s.%s: catalog = (%s, %v), want explicit classification", tc.imported, name, function.Name(), imported, receiver, selector, fact.Disposition, err)
					}
				}
			}
		})
	}
}
