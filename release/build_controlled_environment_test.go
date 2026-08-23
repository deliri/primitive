package release

import (
	"slices"
	"testing"
)

func TestBuildControlledEnvironmentNameHostileBoundaries(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   string
		want bool
	}{
		{name: "PATH is controlled", in: goPathEnvironmentName, want: true},
		{name: "CGO_ENABLED is controlled", in: goEnvironmentNameCGO, want: true},
		{name: "GOOS is controlled", in: goEnvironmentNameOS, want: true},
		{name: "GOARCH is controlled", in: goEnvironmentNameArchitecture, want: true},
		{name: "GOTOOLCHAIN is controlled", in: goEnvironmentNameToolchain, want: true},
		{name: "GOAMD64 is controlled", in: goEnvironmentNameAMD64, want: true},
		{name: "GOARM64 is controlled", in: goEnvironmentNameARM64, want: true},
		{name: "GOENV is controlled", in: goEnvironmentNameConfiguration, want: true},
		{name: "GOFLAGS is controlled", in: goEnvironmentNameFlags, want: true},
		{name: "GOEXPERIMENT is controlled", in: goEnvironmentNameExperiment, want: true},
		{name: "GOFIPS140 is controlled", in: goEnvironmentNameFIPS, want: true},
		{name: "GOWORK is controlled", in: goEnvironmentNameWorkspace, want: true},
		{name: "GOCACHE is preserved for the executor", in: "GOCACHE", want: false},
		{name: "GOMODCACHE is preserved for the executor", in: "GOMODCACHE", want: false},
		{name: "GOPATH is preserved for the executor", in: "GOPATH", want: false},
		{name: "GOTMPDIR is preserved for the executor", in: "GOTMPDIR", want: false},
		{name: "GOROOT is preserved for the executor", in: "GOROOT", want: false},
		{name: "HOME is preserved", in: "HOME", want: false},
		{name: "USER is preserved", in: "USER", want: false},
		{name: "TMPDIR is preserved", in: "TMPDIR", want: false},
		{name: "LANG is preserved", in: "LANG", want: false},
		{name: "TERM is preserved", in: "TERM", want: false},
		{name: "empty name is not controlled", in: ""},
		{name: "space only is not controlled", in: " "},
		{name: "GO prefix alone is not controlled", in: "GO"},
		{name: "lowercase path is not controlled", in: "path"},
		{name: "lowercase goos is not controlled", in: "goos"},
		{name: "title-case Path is not controlled", in: "Path"},
		{name: "leading space on PATH is not controlled", in: " PATH"},
		{name: "trailing space on PATH is not controlled", in: "PATH "},
		{name: "newline after PATH is not controlled", in: "PATH\n"},
		{name: "NUL after PATH is not controlled", in: "PATH\x00"},
		{name: "GOCACHE suffix on GO is not controlled", in: "GOCACHE"},
		{name: "one below PATH first byte is not controlled", in: string([]byte{goPathEnvironmentName[0] - 1}) + goPathEnvironmentName[1:]},
		{name: "one above PATH last byte is not controlled", in: goPathEnvironmentName[:len(goPathEnvironmentName)-1] + string([]byte{goPathEnvironmentName[len(goPathEnvironmentName)-1] + 1})},
		{name: "truncated PATH is not controlled", in: "PAT"},
		{name: "oversize PATH suffix is not controlled", in: "PATHS"},
		{name: "GO111MODULE is not a target control", in: "GO111MODULE"},
		{name: "GOPROXY is not a target control", in: "GOPROXY"},
		{name: "GOSUMDB is not a target control", in: "GOSUMDB"},
		{name: "GODEBUG is not a target control", in: "GODEBUG"},
		{name: "GOMAXPROCS is not a target control", in: "GOMAXPROCS"},
		{name: "GOCOVERDIR is not a target control", in: "GOCOVERDIR"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := buildControlledEnvironmentName(tc.in)
			if got != tc.want {
				t.Fatalf("buildControlledEnvironmentName(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestBuildControlledEnvironmentNameLayerTriadPositiveClosedSetIsReplaced(t *testing.T) {
	t.Parallel()

	for _, name := range buildControlledEnvironmentNames() {
		if !buildControlledEnvironmentName(name) {
			t.Fatalf("buildControlledEnvironmentName(%q) = false, want true", name)
		}
	}
}

func TestBuildControlledEnvironmentNameLayerTriadNegativeExecutorCachesSurvive(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"GOCACHE", "GOMODCACHE", "GOPATH"} {
		if buildControlledEnvironmentName(name) {
			t.Fatalf("buildControlledEnvironmentName(%q) = true, want false", name)
		}
	}
}

func TestBuildControlledEnvironmentNameLayerTriadNeutralEmptyNameIsNotControlled(t *testing.T) {
	t.Parallel()

	if buildControlledEnvironmentName("") {
		t.Fatal("buildControlledEnvironmentName(\"\") = true, want false")
	}
}

func FuzzBuildControlledEnvironmentNameSemanticClosure(f *testing.F) {
	for _, name := range buildControlledEnvironmentNames() {
		f.Add(name)
	}
	f.Add("")
	f.Add("GOCACHE")
	f.Add("GOMODCACHE")
	f.Add("GOPATH")
	f.Add("path")
	f.Fuzz(func(t *testing.T, name string) {
		got := buildControlledEnvironmentName(name)
		want := slices.Contains(buildControlledEnvironmentNames(), name)
		if got != want {
			t.Fatalf("buildControlledEnvironmentName(%q) = %v, want %v", name, got, want)
		}
	})
}
