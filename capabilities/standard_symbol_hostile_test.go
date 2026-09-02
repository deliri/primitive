package capabilities

import (
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
		{name: "negative parser file requires syntax context", importPath: "go/parser", selector: "ParseFile", want: StandardSymbolContextual},
		{name: "neutral filepath Join is pure", importPath: "path/filepath", selector: "Join", want: StandardSymbolPure},
		{name: "neutral SHA256 Sum256 is pure by package", importPath: "crypto/sha256", selector: "Sum256", want: StandardSymbolPure},
		{name: "negative unknown os selector remains unresolved", importPath: "os", selector: "FutureEffect", want: StandardSymbolUnknown},
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
