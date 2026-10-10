package hostfacts

import (
	"fmt"
	"runtime/debug"
	"slices"
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

func TestObserveGoBuildVCSMatchesNativeGoExecutableMetadata(t *testing.T) {
	t.Parallel()
	native, available := debug.ReadBuildInfo()
	got, gotAvailable := ObserveGoBuildVCS()
	if gotAvailable != available {
		t.Fatalf("availability = %t, want native %t", gotAvailable, available)
	}
	var want GoBuildVCS
	if available {
		for _, setting := range native.Settings {
			if setting.Key == "vcs.revision" {
				want.Revision = setting.Value
			}
			if setting.Key == "vcs.modified" {
				want.Modified = setting.Value == "true"
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
