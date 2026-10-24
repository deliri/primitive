package capabilities

import (
	"errors"
	"os"
	"syscall"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gomodule"
)

// These method expressions bind the shared-custody identities to Go's actual
// descriptor APIs. Native lifetime and callback behavior remain their owners'
// behavioral proof; ownership must neither invent nor replace that proof.
var (
	_ func(*os.File) (syscall.RawConn, error)    = (*os.File).SyscallConn
	_ func(syscall.RawConn, func(uintptr)) error = syscall.RawConn.Control
)

func TestOwnsStandardSymbolExactOwnership(t *testing.T) {
	t.Parallel()
	catalog, err := All()
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, imported, receiver, selector string
		owners                             []core.PackageIdentity
	}{
		{name: "file descriptor custody", imported: "os", receiver: "File", selector: "SyscallConn", owners: []core.PackageIdentity{core.PackageFilestore, core.PackageProcess, core.PackageFileLock, core.PackageHostFacts}},
		{name: "raw descriptor custody", imported: "syscall", receiver: "RawConn", selector: "Control", owners: []core.PackageIdentity{core.PackageFilestore, core.PackageProcess, core.PackageFileLock, core.PackageHostFacts}},
		{name: "raw read retains host owner", imported: "syscall", receiver: "RawConn", selector: "Read", owners: []core.PackageIdentity{core.PackageHostFacts}},
		{name: "raw write retains host owner", imported: "syscall", receiver: "RawConn", selector: "Write", owners: []core.PackageIdentity{core.PackageHostFacts}},
		{name: "unheld fd retains host owner", imported: "os", receiver: "File", selector: "Fd", owners: []core.PackageIdentity{core.PackageHostFacts}},
		{name: "environment retains host owner", imported: "os", selector: "Getenv", owners: []core.PackageIdentity{core.PackageHostFacts}},
		{name: "file write retains filesystem owner", imported: "os", receiver: "File", selector: "Write", owners: []core.PackageIdentity{core.PackageFilestore}},
		{name: "process run retains process owner", imported: "os/exec", receiver: "Cmd", selector: "Run", owners: []core.PackageIdentity{core.PackageProcess}},
		{name: "locking retains locking owner", imported: "syscall", selector: "Flock", owners: []core.PackageIdentity{core.PackageFileLock}},
		{name: "transport retains transport owner", imported: "net", selector: "Dial", owners: []core.PackageIdentity{core.PackageExchange}},
		{name: "pure is no native authority", imported: "os", receiver: "File", selector: "Name"},
		{name: "contextual is no native authority", imported: "encoding/json", selector: "Marshal"},
		{name: "unknown is no native authority", imported: "future", receiver: "RawConn", selector: "Control"},
		{name: "same function name is not method", imported: "syscall", selector: "Control"},
		{name: "same method name on another receiver", imported: "os", receiver: "Root", selector: "SyscallConn"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			symbol := ownershipSymbol(t, tc.imported, tc.receiver, tc.selector)
			before, err := ResolveStandardSymbol(symbol)
			if err != nil {
				t.Fatal(err)
			}
			for contract := range core.PrimitiveArchitecture().Packages() {
				path, err := contract.Identity.ImportPath()
				if err != nil {
					t.Fatal(err)
				}
				caller, err := gomodule.ParseImportPath(path)
				if err != nil {
					t.Fatal(err)
				}
				want := false
				for _, owner := range tc.owners {
					want = want || contract.Identity == owner
				}
				got, err := catalog.OwnsStandardSymbol(symbol, caller)
				if err != nil || got != want {
					t.Fatalf("OwnsStandardSymbol(%v,%s) = (%t,%v), want (%t,nil)", symbol, path, got, err, want)
				}
			}
			after, err := ResolveStandardSymbol(symbol)
			if err != nil || !before.Classification.Equal(after.Classification) {
				t.Fatalf("classification after ownership = (%+v,%v), want unchanged %+v", after, err, before)
			}
		})
	}
}

func TestOwnsStandardSymbolRefusesCallerLookalikes(t *testing.T) {
	t.Parallel()
	catalog, err := All()
	if err != nil {
		t.Fatal(err)
	}
	for _, callerText := range []string{
		"example.com/product/filestore",
		"github.com/deliri/primitive/v2026/filestore/child",
		"github.com/deliri/primitive/v2026/filestoreevil",
		"github.com/deliri/primitive/v2026evil/filestore",
		"github.com/deliri/primitive/v2027/filestore",
		"github.com/deliri/primitive/v2026/Filestore",
	} {
		t.Run(callerText, func(t *testing.T) {
			t.Parallel()
			caller, err := gomodule.ParseImportPath(callerText)
			if err != nil {
				t.Fatal(err)
			}
			for _, symbol := range []StandardSymbol{ownershipSymbol(t, "os", "File", "SyscallConn"), ownershipSymbol(t, "syscall", "RawConn", "Control")} {
				got, err := catalog.OwnsStandardSymbol(symbol, caller)
				if err != nil || got {
					t.Fatalf("OwnsStandardSymbol(%v,%s) = (%t,%v), want (false,nil)", symbol, callerText, got, err)
				}
			}
		})
	}
}

func TestOwnsStandardSymbolRejectsInvalidInputs(t *testing.T) {
	t.Parallel()
	catalog, err := All()
	if err != nil {
		t.Fatal(err)
	}
	symbol := ownershipSymbol(t, "os", "File", "SyscallConn")
	caller, err := gomodule.ParseImportPath("github.com/deliri/primitive/v2026/filestore")
	if err != nil {
		t.Fatal(err)
	}
	invalidReceiver := SymbolName{}
	invalidSymbol := symbol
	invalidSymbol.Receiver = &invalidReceiver
	cases := []struct {
		name    string
		catalog Catalog
		symbol  StandardSymbol
		caller  gomodule.ImportPath
	}{
		{name: "zero catalog cannot grant native authority", symbol: symbol, caller: caller},
		{name: "zero symbol cannot grant native authority", catalog: catalog, caller: caller},
		{name: "zero caller cannot grant native authority", catalog: catalog, symbol: symbol},
		{name: "invalid receiver cannot grant native authority", catalog: catalog, symbol: invalidSymbol, caller: caller},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := tc.catalog.OwnsStandardSymbol(tc.symbol, tc.caller)
			if got || !errors.Is(err, core.ErrCapabilitiesContract) {
				t.Fatalf("OwnsStandardSymbol() = (%t,%v), want false and ErrCapabilitiesContract", got, err)
			}
		})
	}
}

func ownershipSymbol(t testing.TB, imported, receiver, selector string) StandardSymbol {
	t.Helper()
	path, err := gomodule.ParseImportPath(imported)
	if err != nil {
		t.Fatal(err)
	}
	name, err := ParseSymbolName(selector)
	if err != nil {
		t.Fatal(err)
	}
	symbol := StandardSymbol{ImportPath: path, Selector: name}
	if receiver != "" {
		name, err := ParseSymbolName(receiver)
		if err != nil {
			t.Fatal(err)
		}
		symbol.Receiver = &name
	}
	return symbol
}
