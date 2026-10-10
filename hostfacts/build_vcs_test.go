package hostfacts

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"slices"
	"strings"
	"testing"
)

func TestGoBuildVCSProjectionPreservesCompilerFactsAcrossNativeSettings(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		settings []debug.BuildSetting
		want     GoBuildVCS
	}{
		{name: "absent"},
		{name: "revision only", settings: []debug.BuildSetting{{Key: "vcs.revision", Value: "ABC123"}}, want: GoBuildVCS{Revision: "ABC123"}},
		{name: "modified only", settings: []debug.BuildSetting{{Key: "vcs.modified", Value: "true"}}, want: GoBuildVCS{Modified: true}},
		{name: "clean", settings: []debug.BuildSetting{{Key: "vcs.revision", Value: "abc"}, {Key: "vcs.modified", Value: "false"}}, want: GoBuildVCS{Revision: "abc"}},
		{name: "dirty", settings: []debug.BuildSetting{{Key: "vcs.revision", Value: "abc"}, {Key: "vcs.modified", Value: "true"}}, want: GoBuildVCS{Revision: "abc", Modified: true}},
		{name: "revision bytes preserved", settings: []debug.BuildSetting{{Key: "vcs.revision", Value: " \tABC\n"}}, want: GoBuildVCS{Revision: " \tABC\n"}},
		{name: "modified uppercase is not true", settings: []debug.BuildSetting{{Key: "vcs.modified", Value: "TRUE"}}},
		{name: "modified whitespace is not true", settings: []debug.BuildSetting{{Key: "vcs.modified", Value: " true "}}},
		{name: "modified number is not true", settings: []debug.BuildSetting{{Key: "vcs.modified", Value: "1"}}},
		{name: "near keys are unrelated", settings: []debug.BuildSetting{{Key: "vcs.revision ", Value: "wrong"}, {Key: "VCS.modified", Value: "true"}}},
		{name: "last revision includes empty", settings: []debug.BuildSetting{{Key: "vcs.revision", Value: "old"}, {Key: "vcs.revision", Value: ""}}},
		{name: "last modified includes clean", settings: []debug.BuildSetting{{Key: "vcs.modified", Value: "true"}, {Key: "vcs.modified", Value: "false"}}},
	}
	layouts := []struct {
		name          string
		before, after int
	}{
		{name: "bare"}, {name: "leading unrelated", before: 1}, {name: "trailing unrelated", after: 1},
		{name: "buried in compiler settings", before: 32, after: 32}, {name: "after large unrelated extent", before: 4096},
	}
	for _, tc := range cases {
		for _, layout := range layouts {
			t.Run(tc.name+"/"+layout.name, func(t *testing.T) {
				t.Parallel()
				settings := make([]debug.BuildSetting, 0, layout.before+len(tc.settings)+layout.after)
				for i := 0; i < layout.before; i++ {
					settings = append(settings, debug.BuildSetting{Key: fmt.Sprintf("unrelated.%d", i), Value: "true"})
				}
				settings = append(settings, tc.settings...)
				for i := 0; i < layout.after; i++ {
					settings = append(settings, debug.BuildSetting{Key: fmt.Sprintf("ignored.%d", i), Value: "wrong"})
				}
				original := slices.Clone(settings)
				if !slices.Equal(settings, original) {
					t.Fatalf("projection changed native settings: %+v", settings)
				}
			})
		}
	}
}

func TestObserveGoBuildMetadataMatchesNativeGoExecutableMetadata(t *testing.T) {
	t.Parallel()
	native, available := debug.ReadBuildInfo()
	got, gotAvailable := ObserveGoBuildMetadata()
	if gotAvailable != available {
		t.Fatalf("availability = %t, want native %t", gotAvailable, available)
	}
	want := GoBuildMetadata{GoVersion: runtime.Version()}
	if available {
		want.ModulePath = native.Main.Path
		want.GoVersion = native.GoVersion
		for _, setting := range native.Settings {
			if setting.Key == "vcs.revision" {
				want.VCS.Revision = setting.Value
			}
			if setting.Key == "vcs.modified" {
				want.VCS.Modified = setting.Value == "true"
			}
		}
	}
	if got != want {
		t.Fatalf("observation = %+v, want actual native facts %+v", got, want)
	}
}

func FuzzGoBuildVCSProjectionPreservesUnboundedFactBytes(f *testing.F) {
	for _, revision := range []string{"", "abc", " ABC\n", "\x00", "vcs.modified"} {
		for _, modified := range []string{"true", "false", "TRUE", " true ", ""} {
			f.Add(revision, modified, uint8(0))
			f.Add(revision, modified, uint8(1))
		}
	}
	f.Fuzz(func(t *testing.T, revision, modified string, placement uint8) {
		planted := debug.BuildSetting{Key: "unrelated", Value: revision}
		settings := []debug.BuildSetting{{Key: "vcs.revision", Value: "overridden"}, {Key: "vcs.modified", Value: "true"}}
		if placement&1 == 0 {
			settings = append(settings, planted)
		}
		settings = append(settings, debug.BuildSetting{Key: "vcs.revision", Value: revision}, debug.BuildSetting{Key: "vcs.modified", Value: modified})
		if placement&1 != 0 {
			settings = append(settings, planted)
		}
		want := GoBuildVCS{Revision: revision, Modified: modified == "true"}
		if got := goBuildVCSFromSettings(settings); got != want {
			t.Fatalf("projection = %+v, want input facts %+v", got, want)
		}
	})
}

func BenchmarkGoBuildVCSFixedProjection(b *testing.B) {
	for _, extent := range []int{1, 4096} {
		b.Run(fmt.Sprint(extent), func(b *testing.B) {
			settings := make([]debug.BuildSetting, extent+2)
			for i := range extent {
				settings[i] = debug.BuildSetting{Key: "unrelated", Value: "true"}
			}
			settings[extent] = debug.BuildSetting{Key: "vcs.revision", Value: "abc"}
			settings[extent+1] = debug.BuildSetting{Key: "vcs.modified", Value: "true"}
			var got GoBuildVCS
			var observed uint64
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				got = goBuildVCSFromSettings(settings)
				observed++
			}
			b.StopTimer()
			if observed != uint64(b.N) || got != (GoBuildVCS{Revision: "abc", Modified: true}) {
				b.Fatalf("observable projection = %+v after %d operations", got, observed)
			}
		})
	}
}

func TestGoBuildMetadataProjectsMainModuleWithoutDependencyInventory(t *testing.T) {
	t.Parallel()
	paths := []struct{ name, value string }{
		{name: "absent"}, {name: "main module", value: "github.com/deliri/primitive/v2026"},
		{name: "foreign module", value: "example.com/other"}, {name: "uppercase retained", value: "EXAMPLE.com/Other"},
		{name: "whitespace retained", value: " \tmodule\n"}, {name: "embedded nul retained", value: "module\x00path"},
		{name: "unicode retained", value: "example.com/é"}, {name: "relative compiler data retained", value: "./module"},
		{name: "version suffix retained", value: "example.com/module/v2026"}, {name: "replacement spelling retained", value: "replacement"},
		{name: "go tool command", value: "cmd"}, {name: "single rune", value: "x"},
		{name: "long fact extent", value: strings.Repeat("x", 4096)},
		{name: "larger fact extent", value: strings.Repeat("x", 65536)},
		{name: "large fact has no projection quota", value: strings.Repeat("x", 131072)},
	}
	vcs := []struct {
		name, revision, modified string
		want                     GoBuildVCS
	}{
		{name: "empty"}, {name: "revision clean", revision: "abc", modified: "false", want: GoBuildVCS{Revision: "abc"}},
		{name: "revision dirty", revision: "ABC", modified: "true", want: GoBuildVCS{Revision: "ABC", Modified: true}},
		{name: "dirty without revision", modified: "true", want: GoBuildVCS{Modified: true}},
	}
	for _, path := range paths {
		for _, facts := range vcs {
			t.Run(path.name+"/"+facts.name, func(t *testing.T) {
				t.Parallel()
				native := debug.BuildInfo{GoVersion: "go1.27.2", Path: "example.com/command", Main: debug.Module{Path: path.value, Replace: &debug.Module{Path: "example.com/replacement"}}, Deps: []*debug.Module{{Path: "example.com/dependency"}}, Settings: []debug.BuildSetting{{Key: "vcs.revision", Value: facts.revision}, {Key: "vcs.modified", Value: facts.modified}}}
				want := GoBuildMetadata{ModulePath: path.value, GoVersion: "go1.27.2", VCS: facts.want}
				if got := goBuildMetadataFromInfo(&native); got != want {
					t.Fatalf("metadata = %+v, want planted main-module facts %+v", got, want)
				}
				if native.Main.Path != path.value || native.Main.Replace.Path != "example.com/replacement" || native.Deps[0].Path != "example.com/dependency" {
					t.Fatal("projection mutated native module facts")
				}
			})
		}
	}
}

func BenchmarkGoBuildMetadataFixedProjection(b *testing.B) {
	for _, extent := range []int{1, 4096} {
		b.Run(fmt.Sprint(extent), func(b *testing.B) {
			native := debug.BuildInfo{GoVersion: "go1.27.2", Main: debug.Module{Path: "example.com/main"}, Settings: make([]debug.BuildSetting, extent+2)}
			for i := range extent {
				native.Settings[i] = debug.BuildSetting{Key: "unrelated", Value: "true"}
			}
			native.Settings[extent] = debug.BuildSetting{Key: "vcs.revision", Value: "abc"}
			native.Settings[extent+1] = debug.BuildSetting{Key: "vcs.modified", Value: "true"}
			var got GoBuildMetadata
			var observed uint64
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				got = goBuildMetadataFromInfo(&native)
				observed++
			}
			b.StopTimer()
			want := GoBuildMetadata{GoVersion: "go1.27.2", ModulePath: "example.com/main", VCS: GoBuildVCS{Revision: "abc", Modified: true}}
			if observed != uint64(b.N) || got != want {
				b.Fatalf("observable metadata = %+v after %d operations, want %+v", got, observed, want)
			}
		})
	}
}
