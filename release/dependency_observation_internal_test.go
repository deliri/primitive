package release

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestDecodeBuildDependenciesLayerTriadPressuresGoListProtocol(t *testing.T) {
	t.Parallel()

	const (
		mainPath = "example.com/product"
		zeroSum  = "h1:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
		oneSum   = "h1:AQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
	)
	main := goListPackageFixture(mainPath, "", "", true)
	depA := goListPackageFixture("example.com/a", "v1.2.3", zeroSum, false)
	depB := goListPackageFixture("example.com/b/v2", "v2.0.0-20260804010203-0123456789ab", oneSum, false)
	standard := `{"ImportPath":"io","Standard":true}`
	cases := []struct {
		wantErr   error
		name      string
		input     string
		wantMain  string
		wantCount int
	}{
		{name: "positive main package only", input: main, wantMain: mainPath},
		{name: "positive standard package before main", input: standard + main, wantMain: mainPath},
		{name: "positive one dependency", input: depA + main, wantMain: mainPath, wantCount: 1},
		{name: "positive repeated package from one dependency", input: depA + depA + main, wantMain: mainPath, wantCount: 1},
		{name: "positive reverse dependency order is canonicalized", input: depB + depA + main, wantMain: mainPath, wantCount: 2},
		{name: "positive forward dependency order remains canonical", input: depA + depB + main, wantMain: mainPath, wantCount: 2},
		{name: "positive repeated main module is one identity", input: main + main, wantMain: mainPath},
		{name: "positive ignored documented fields do not alter facts", input: `{"ImportPath":"io","Standard":true,"Dir":"/ignored","GoFiles":["ignored.go"]}` + main, wantMain: mainPath},
		{name: "positive pseudo-version is retained", input: depB + main, wantMain: mainPath, wantCount: 1},
		{name: "positive major-version module path is retained", input: main + depB, wantMain: mainPath, wantCount: 1},
		{name: "negative empty stream", wantErr: core.ErrReleaseContract},
		{name: "negative empty package object", input: `{}`, wantErr: core.ErrReleaseContract},
		{name: "negative truncated object", input: `{"ImportPath":`, wantErr: core.ErrReleaseContract},
		{name: "negative malformed object", input: `{not-json}`, wantErr: core.ErrReleaseContract},
		{name: "negative incomplete package", input: `{"ImportPath":"example.com/p","Incomplete":true}` + main, wantErr: core.ErrReleaseContract},
		{name: "negative package error", input: `{"ImportPath":"example.com/p","Error":{"Err":"broken"}}` + main, wantErr: core.ErrReleaseContract},
		{name: "negative nonstandard package without module", input: `{"ImportPath":"example.com/p"}` + main, wantErr: core.ErrReleaseContract},
		{name: "negative standard package with module", input: strings.Replace(depA, `"ImportPath"`, `"Standard":true,"ImportPath"`, 1) + main, wantErr: core.ErrReleaseContract},
		{name: "negative replaced dependency", input: strings.Replace(depA, `"Main":false`, `"Main":false,"Replace":{"Path":"example.com/local"}`, 1) + main, wantErr: core.ErrReleaseContract},
		{name: "negative dependency missing version", input: goListPackageFixture("example.com/a", "", zeroSum, false) + main, wantErr: core.ErrReleaseContract},
		{name: "negative dependency missing sum", input: goListPackageFixture("example.com/a", "v1.2.3", "", false) + main, wantErr: core.ErrReleaseContract},
		{name: "negative dependency malformed sum", input: goListPackageFixture("example.com/a", "v1.2.3", "h1:not-base64", false) + main, wantErr: core.ErrReleaseContract},
		{name: "negative dependency malformed path", input: goListPackageFixture("-flag/path", "v1.2.3", zeroSum, false) + main, wantErr: core.ErrReleaseContract},
		{name: "negative distinct main modules", input: main + goListPackageFixture("example.com/other", "", "", true), wantErr: core.ErrReleaseContract},
		{name: "negative conflicting repeated module version", input: depA + goListPackageFixture("example.com/a", "v1.2.4", zeroSum, false) + main, wantErr: core.ErrReleaseContract},
		{name: "negative conflicting repeated module sum", input: depA + goListPackageFixture("example.com/a", "v1.2.3", oneSum, false) + main, wantErr: core.ErrReleaseContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			observed := &dependencyObservation{}
			err := decodeBuildDependencies(strings.NewReader(tc.input), observed)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("decodeBuildDependencies() error = %v, want errors.Is(..., %v)", err, tc.wantErr)
				}
				if *observed != (dependencyObservation{}) {
					t.Fatalf("decodeBuildDependencies() = %v, want zero facts on rejection", *observed)
				}
				return
			}
			if err != nil {
				t.Fatalf("decodeBuildDependencies() error = %v, want nil", err)
			}
			if observed.main.String() != tc.wantMain || observed.count != tc.wantCount {
				t.Fatalf("decodeBuildDependencies() = (%q, %d modules), want (%q, %d)", observed.main.String(), observed.count, tc.wantMain, tc.wantCount)
			}
			for index := 1; index < observed.count; index++ {
				if observed.modules[index-1].Path().String() >= observed.modules[index].Path().String() {
					t.Fatalf("module slots %d and %d are not path-sorted", index-1, index)
				}
			}
		})
	}
}

// TestDecodeBuildDependenciesReplacesThePriorTargetObservation proves the
// reusable fixed buffer represents exactly one target. Retaining the preceding
// target's modules would make a later target appear to contain dependencies it
// never reported and could hide a broken target observation in the final union.
func TestDecodeBuildDependenciesReplacesThePriorTargetObservation(t *testing.T) {
	t.Parallel()

	const (
		mainPath  = "example.com/product"
		moduleSum = "h1:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
	)
	observed := &dependencyObservation{}
	first := goListPackageFixture("example.com/first", "v1.2.3", moduleSum, false) +
		goListPackageFixture(mainPath, "", "", true)
	if err := decodeBuildDependencies(strings.NewReader(first), observed); err != nil {
		t.Fatalf("decodeBuildDependencies(first target) error = %v, want nil", err)
	}
	if observed.count != 1 {
		t.Fatalf("first target module count = %d, want 1", observed.count)
	}
	second := goListPackageFixture(mainPath, "", "", true)
	if err := decodeBuildDependencies(strings.NewReader(second), observed); err != nil {
		t.Fatalf("decodeBuildDependencies(second target) error = %v, want nil", err)
	}
	if observed.main.String() != mainPath || observed.count != 0 {
		t.Fatalf("second target observation = (%q, %d modules), want (%q, 0)",
			observed.main.String(), observed.count, mainPath)
	}
}

func goListPackageFixture(path, version, sum string, main bool) string {
	return fmt.Sprintf(
		`{"ImportPath":%q,"Module":{"Path":%q,"Version":%q,"Sum":%q,"Main":%t}}`,
		path+"/package", path, version, sum, main,
	)
}
