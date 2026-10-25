package runprotocol_test

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/runprotocol"
)

type nativeFuzzSelectionFixture struct {
	directory string
	pattern   string
	targets   [3]string
}

// Native go test is the oracle: for each -fuzz pattern, the targets the
// pattern selects are exactly the ones Go fuzzes. One selection fuzzes it;
// none fuzzes nothing and exits zero; several, or an element that does not
// compile, exits one without fuzzing. The expected selections are also written
// out independently, so the matcher cannot agree with itself alone.
func TestGoTestPatternMatchesNativeFuzzSelection(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		pattern string
		want    []string
		invalid bool
	}{
		{pattern: "FuzzF", want: []string{"FuzzF"}},
		{pattern: "[F/]uzzF", want: []string{"FuzzF"}},
		{pattern: "FuzzF(/seed)?", want: []string{"FuzzF"}},
		{pattern: "Other/seed|FuzzF", want: []string{"FuzzF"}},
		{pattern: "^FuzzF$/seed", want: []string{"FuzzF"}},
		{pattern: `Fuzz\/F`},
		{pattern: "Fuzz F$", want: []string{"Fuzz_F"}},
		{pattern: "]FuzzF"},
		{pattern: "Missing"},
		{pattern: "Missing/FuzzG"},
		{pattern: "(F)uzzF/x", want: []string{"FuzzF"}},
		{pattern: "]?FuzzF/x", want: []string{"FuzzF"}},
		{pattern: "^Fuzz[FG]$", want: []string{"FuzzF", "FuzzG"}},
		{pattern: "^FuzzF$|^FuzzG$", want: []string{"FuzzF", "FuzzG"}},
		{pattern: "(FuzzF", invalid: true},
		{pattern: "FuzzF/(", invalid: true},
	} {
		t.Run(row.pattern, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			fixture := nativeFuzzSelectionFixture{directory: dir, pattern: row.pattern, targets: [3]string{"FuzzF", "FuzzG", "Fuzz_F"}}
			pattern, err := runprotocol.ParseGoTestPattern(row.pattern)
			if row.invalid != (err != nil) || row.invalid && (!errors.Is(err, core.ErrGoToolchainContract) || pattern != (runprotocol.GoTestPattern{})) {
				t.Fatalf("ParseGoTestPattern(%q) = (%v,%v), want invalid %t", row.pattern, pattern, err, row.invalid)
			}
			var predicted []string
			for _, target := range fixture.targets {
				selected, err := pattern.SelectsTopLevel(target)
				if err != nil && !row.invalid {
					t.Fatal(err)
				}
				if selected {
					predicted = append(predicted, target)
				}
			}
			if !row.invalid && !slices.Equal(predicted, row.want) {
				t.Fatalf("pattern %q selects %q, want %q", row.pattern, predicted, row.want)
			}
			fuzzed, exit := fixture.run(t)
			wantExit := 0
			if row.invalid || len(row.want) > 1 {
				wantExit = 1
			}
			wantFuzzed := row.want
			if wantExit != 0 {
				wantFuzzed = nil
			}
			if exit != wantExit || !slices.Equal(fuzzed, wantFuzzed) {
				t.Fatalf("native go test -fuzz=%q fuzzed %q exit %d, want %q exit %d", row.pattern, fuzzed, exit, wantFuzzed, wantExit)
			}
		})
	}
}

// run executes go test -fuzz once over the fixture and reports
// which targets completed, by Primitive's own native frame reader.
func (fixture nativeFuzzSelectionFixture) run(t *testing.T) ([]string, int) {
	t.Helper()
	body := "package scope\nimport \"testing\"\n"
	for _, target := range fixture.targets {
		body += "func " + target + "(f *testing.F) { f.Add(1); f.Fuzz(func(t *testing.T, value int) {}) }\n"
	}
	for _, file := range [...]struct{ name, body string }{
		{"go.mod", "module example.invalid/patternselection\n\ngo 1.27.2\n"},
		{"scope_test.go", body},
	} {
		if err := os.WriteFile(filepath.Join(fixture.directory, file.name), []byte(file.body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	stdout, err := os.Create(filepath.Join(fixture.directory, "stdout"))
	if err != nil {
		t.Fatal(err)
	}
	defer stdout.Close()
	cmd := exec.CommandContext(t.Context(), "go", "test", "-mod=readonly", "-count=1", "-vet=off", "-v", "-run=^$", "-fuzz="+fixture.pattern, "-fuzztime=1x", "-parallel=1", ".")
	cmd.Dir, cmd.Stdout, cmd.Stderr = fixture.directory, stdout, io.Discard
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOMAXPROCS=1")
	runErr := cmd.Run()
	if cmd.ProcessState == nil {
		t.Fatalf("native go test did not run: %v", runErr)
	}
	var fuzzed []string
	for _, name := range fixture.targets {
		if _, err := stdout.Seek(0, io.SeekStart); err != nil {
			t.Fatal(err)
		}
		got, err := runprotocol.ObserveGoFuzzCompletion(t.Context(), runprotocol.GoFuzzCompletionRequest{Source: stdout, Target: fuzzCompletionTarget(t, name)})
		if err != nil {
			t.Fatal(err)
		}
		if got.Completion() != runprotocol.GoFuzzCompletionAbsent {
			fuzzed = append(fuzzed, name)
		}
	}
	return fuzzed, cmd.ProcessState.ExitCode()
}
