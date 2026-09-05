package capabilities

import (
	"errors"
	"github.com/deliri/primitive/v2026/core"
	"testing"

	"github.com/deliri/primitive/v2026/gomodule"
)

func TestStandardSymbolOwnershipLayerTriad(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name          string
		importPath    string
		selector      string
		want          StandardSymbolDisposition
		wantEffect    Effect
		wantSecondary Effect
	}{
		{name: "positive os ReadFile belongs to filesystem", importPath: "os", selector: "ReadFile", want: StandardSymbolEffect, wantEffect: EffectFilesystem},
		{name: "positive os Getenv belongs to host", importPath: "os", selector: "Getenv", want: StandardSymbolEffect, wantEffect: EffectHost},
		{name: "positive HTTP Get belongs to transport", importPath: "net/http", selector: "Get", want: StandardSymbolEffect, wantEffect: EffectTransport},
		{name: "positive HTTP ServeFile retains transport and filesystem", importPath: "net/http", selector: "ServeFile", want: StandardSymbolEffect, wantEffect: EffectTransport, wantSecondary: EffectFilesystem},
		{name: "positive process exit belongs to process", importPath: "os", selector: "Exit", want: StandardSymbolEffect, wantEffect: EffectProcess},
		{name: "positive time Now belongs to time", importPath: "time", selector: "Now", want: StandardSymbolEffect, wantEffect: EffectTime},
		{name: "positive syscall Flock belongs to locking", importPath: "syscall", selector: "Flock", want: StandardSymbolEffect, wantEffect: EffectLocking},
		{name: "positive unix Flock belongs to locking", importPath: "golang.org/x/sys/unix", selector: "Flock", want: StandardSymbolEffect, wantEffect: EffectLocking},

		{name: "directory copy belongs to filesystem", importPath: "os", selector: "CopyFS", want: StandardSymbolEffect, wantEffect: EffectFilesystem},
		{name: "file timestamps belong to filesystem", importPath: "os", selector: "Chtimes", want: StandardSymbolEffect, wantEffect: EffectFilesystem},
		{name: "rooted open belongs to filesystem", importPath: "os", selector: "OpenInRoot", want: StandardSymbolEffect, wantEffect: EffectFilesystem},
		{name: "pipe creation belongs to process", importPath: "os", selector: "Pipe", want: StandardSymbolEffect, wantEffect: EffectProcess},
		{name: "temporary directory selection observes host configuration", importPath: "os", selector: "TempDir", want: StandardSymbolEffect, wantEffect: EffectHost},
		{name: "elapsed time observes current time", importPath: "time", selector: "Since", want: StandardSymbolEffect, wantEffect: EffectTime},
		{name: "remaining time observes current time", importPath: "time", selector: "Until", want: StandardSymbolEffect, wantEffect: EffectTime},
		{name: "timezone lookup observes host data", importPath: "time", selector: "LoadLocation", want: StandardSymbolEffect, wantEffect: EffectHost},
		{name: "command construction can search executable paths", importPath: "os/exec", selector: "Command", want: StandardSymbolEffect, wantEffect: EffectProcess},
		{name: "future process function is not assumed effectful", importPath: "os/exec", selector: "FutureCommand", want: StandardSymbolUnresolved},
		{name: "negative parser file requires syntax context", importPath: "go/parser", selector: "ParseFile", want: StandardSymbolContextual},
		{name: "neutral filepath Join is pure", importPath: "path/filepath", selector: "Join", want: StandardSymbolPure},
		{name: "neutral SHA256 Sum256 is pure by package", importPath: "crypto/sha256", selector: "Sum256", want: StandardSymbolPure},
		{name: "negative unknown os selector remains unresolved", importPath: "os", selector: "FutureEffect", want: StandardSymbolUnresolved},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			path, err := gomodule.ParseImportPath(tc.importPath)
			if err != nil {
				t.Fatalf("gomodule.ParseImportPath() error = %v, want nil", err)
			}
			selector, err := ParseSymbolName(tc.selector)
			if err != nil {
				t.Fatalf("capabilities.ParseSymbolName() error = %v, want nil", err)
			}
			got, gotErr := ResolveStandardSymbol(StandardSymbol{ImportPath: path, Selector: selector})
			if gotErr != nil {
				t.Fatalf("ResolveStandardSymbol() error = %v, want nil", gotErr)
			}
			if err := got.Validate(); err != nil {
				t.Fatalf("ResolveStandardSymbol().Validate() error = %v, want nil", err)
			}
			secondary := EffectUnknown
			if len(got.Secondary) == 1 {
				secondary = got.Secondary[0]
			}
			if got.Disposition != tc.want || got.Effect != tc.wantEffect || secondary != tc.wantSecondary {
				t.Fatalf("ResolveStandardSymbol() = disposition:%v effect:%v secondary:%v, want %v/%v/%v", got.Disposition, got.Effect, secondary, tc.want, tc.wantEffect, tc.wantSecondary)
			}
		})
	}
}

func TestStandardSymbolOwnershipMutationPairsChangeTheResolvedFact(t *testing.T) {
	t.Parallel()

	resolve := func(t *testing.T, importPath, selectorText string) StandardSymbolFact {
		t.Helper()
		path, err := gomodule.ParseImportPath(importPath)
		if err != nil {
			t.Fatalf("gomodule.ParseImportPath(%q) error = %v, want nil", importPath, err)
		}
		selector, err := ParseSymbolName(selectorText)
		if err != nil {
			t.Fatalf("ParseSymbolName(%q) error = %v, want nil", selectorText, err)
		}
		got, gotErr := ResolveStandardSymbol(StandardSymbol{ImportPath: path, Selector: selector})
		if gotErr != nil {
			t.Fatalf("ResolveStandardSymbol(%s.%s) error = %v, want nil", importPath, selectorText, gotErr)
		}
		return got
	}

	t.Run("changing only the os selector changes filesystem ownership to host ownership", func(t *testing.T) {
		t.Parallel()

		baseline := resolve(t, "os", "ReadFile")
		mutated := resolve(t, "os", "Getenv")
		if baseline.Symbol.ImportPath != mutated.Symbol.ImportPath || baseline.Symbol.Selector == mutated.Symbol.Selector {
			t.Fatalf("selector mutation stable facts = (%v, %v), want same import and changed selector", baseline.Symbol, mutated.Symbol)
		}
		if baseline.Disposition != StandardSymbolEffect || mutated.Disposition != StandardSymbolEffect ||
			baseline.Effect != EffectFilesystem || mutated.Effect != EffectHost {
			t.Fatalf("selector mutation facts = (%v/%v, %v/%v), want effect/filesystem and effect/host", baseline.Disposition, baseline.Effect, mutated.Disposition, mutated.Effect)
		}
	})

	t.Run("changing only the net http selector changes transport ownership to pure", func(t *testing.T) {
		t.Parallel()

		baseline := resolve(t, "net/http", "Get")
		mutated := resolve(t, "net/http", "CanonicalHeaderKey")
		if baseline.Symbol.ImportPath != mutated.Symbol.ImportPath || baseline.Symbol.Selector == mutated.Symbol.Selector {
			t.Fatalf("selector mutation stable facts = (%v, %v), want same import and changed selector", baseline.Symbol, mutated.Symbol)
		}
		if baseline.Disposition != StandardSymbolEffect || baseline.Effect != EffectTransport ||
			mutated.Disposition != StandardSymbolPure || mutated.Effect != EffectUnknown {
			t.Fatalf("selector mutation facts = (%v/%v, %v/%v), want effect/transport and pure/unknown", baseline.Disposition, baseline.Effect, mutated.Disposition, mutated.Effect)
		}
	})
}

func TestStandardSymbolReceiverOwnershipLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, imported, receiver, selector string
		want                               StandardSymbolDisposition
		effect                             Effect
	}{
		{"file close is a filesystem effect", "os", "File", "Close", StandardSymbolEffect, EffectFilesystem},
		{"file name is a pure coordinate", "os", "File", "Name", StandardSymbolPure, EffectUnknown},
		{"unknown receiver cannot inherit the function classification", "os", "FutureFile", "ReadFile", StandardSymbolUnresolved, EffectUnknown},
		{"root metadata is a filesystem effect", "os", "Root", "Stat", StandardSymbolEffect, EffectFilesystem},
		{"process wait owns process observation", "os", "Process", "Wait", StandardSymbolEffect, EffectProcess},
		{"command run owns execution", "os/exec", "Cmd", "Run", StandardSymbolEffect, EffectProcess},
		{"command environment owns host observation", "os/exec", "Cmd", "Environ", StandardSymbolEffect, EffectHost},
		{"command rendering does not execute", "os/exec", "Cmd", "String", StandardSymbolPure, EffectUnknown},
		{"HTTP client performs transport", "net/http", "Client", "Do", StandardSymbolEffect, EffectTransport},
		{"HTTP header formatting is pure", "net/http", "Header", "Get", StandardSymbolPure, EffectUnknown},
		{"connection write performs transport", "net", "Conn", "Write", StandardSymbolEffect, EffectTransport},
		{"timer stop changes the clock facility", "time", "Timer", "Stop", StandardSymbolEffect, EffectTime},
		{"time formatting is pure", "time", "Time", "String", StandardSymbolPure, EffectUnknown},
		{"descriptor control owns the host boundary", "syscall", "RawConn", "Control", StandardSymbolEffect, EffectHost},
		{"unknown method stays unknown", "os", "File", "FutureMethod", StandardSymbolUnresolved, EffectUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			imported, importErr := gomodule.ParseImportPath(tc.imported)
			receiver, receiverErr := ParseSymbolName(tc.receiver)
			selector, selectorErr := ParseSymbolName(tc.selector)
			if err := errors.Join(importErr, receiverErr, selectorErr); err != nil {
				t.Fatalf("symbol fixture error = %v, want nil", err)
			}
			request := StandardSymbol{ImportPath: imported, Selector: selector, Receiver: &receiver}
			got, err := ResolveStandardSymbol(request)
			if err != nil || got.Disposition != tc.want || got.Effect != tc.effect || len(got.Secondary) != 0 {
				t.Fatalf("ResolveStandardSymbol(%s.%s.%s) = (%v/%v, %v), want (%v/%v, nil)", tc.imported, tc.receiver, tc.selector, got.Disposition, got.Effect, err, tc.want, tc.effect)
			}
			if err := got.Validate(); err != nil {
				t.Fatalf("resolved fact Validate() error = %v, want nil", err)
			}
		})
	}
	t.Run("present zero receiver is refused", func(t *testing.T) {
		t.Parallel()
		imported, err := gomodule.ParseImportPath("os")
		if err != nil {
			t.Fatal(err)
		}
		selector, err := ParseSymbolName("Open")
		if err != nil {
			t.Fatal(err)
		}
		receiver := SymbolName{}
		got, err := ResolveStandardSymbol(StandardSymbol{ImportPath: imported, Selector: selector, Receiver: &receiver})
		if !errors.Is(err, core.ErrCapabilitiesContract) || got.Disposition != StandardSymbolUnknown || got.Effect != EffectUnknown || len(got.Secondary) != 0 {
			t.Fatalf("ResolveStandardSymbol(zero receiver) = (%v, %v), want zero and %v", got, err, core.ErrCapabilitiesContract)
		}
	})
}
