package gotoolchain

import (
	"errors"
	"strconv"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/runnercontrol"
)

func TestVetAnalyzerNativeLabelClosure(t *testing.T) {
	t.Parallel()
	for analyzer := VetAnalyzerLostCancel; analyzer <= VetAnalyzerCopyLocks; analyzer++ {
		t.Run(analyzer.String(), func(t *testing.T) {
			t.Parallel()
			got, err := ParseVetAnalyzer(analyzer.String())
			if err != nil || got != analyzer || got.Validate() != nil {
				t.Fatalf("native analyzer=(%v,%v),want%v,nil", got, err, analyzer)
			}
		})
	}
	for _, text := range []string{"", "future", "CopyLocks", "copylocks ", "--copylocks"} {
		t.Run(strconv.Quote(text), func(t *testing.T) {
			t.Parallel()
			got, err := ParseVetAnalyzer(text)
			if got != VetAnalyzerUnknown || !errors.Is(err, core.ErrGoToolchainContract) {
				t.Fatalf("unknown analyzer=(%v,%v),want zero,typed refusal", got, err)
			}
		})
	}
}

func TestObserveVetNativeCopyLocksLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, source string
		want         int
	}{
		{name: "native lock copy", source: "package subject\nimport \"sync\"\ntype S struct{sync.Mutex}\nfunc F(){var original S;cloned:=original;_=&cloned}", want: 1},
		{name: "pointer transfer is neutral", source: "package subject\nimport \"sync\"\ntype S struct{sync.Mutex}\nfunc F(){var original S;cloned:=&original;_=cloned}"},
		{name: "custom name cannot invent a native lock", source: "package subject\ntype Mutex struct{}\ntype S struct{Mutex}\nfunc F(){var original S;cloned:=S(original);_=&cloned}"},
		{name: "type alias retains native lock identity", source: "package subject\nimport \"sync\"\ntype mutex=sync.Mutex\ntype S struct{mu mutex}\nfunc F(){var original S;cloned:=original;_=&cloned}", want: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			compiler, request := nativeVetFixture(t, t.TempDir(), tc.source)
			request.Analyzers = []VetAnalyzer{VetAnalyzerCopyLocks}
			count := 0
			request.Visit = func(got runnercontrol.GoVetObservation) error {
				count++
				analyzer, err := ParseVetAnalyzer(got.Analyzer)
				if err != nil || analyzer != VetAnalyzerCopyLocks || got.Kind != runnercontrol.GoVetObservationDiagnostic {
					t.Fatalf("native lock observation=%+v,%v", got, err)
				}
				_, err = got.SourcePosition()
				return err
			}
			receipt, err := compiler.ObserveVet(t.Context(), request)
			if err != nil || receipt.Validate() != nil || receipt.ExitCode != 0 || count != tc.want {
				t.Fatalf("native copylocks=(%+v,%v,%d),want successful process and%d observations", receipt, err, count, tc.want)
			}
		})
	}
}

func FuzzVetAnalyzerNativeLabelClosure(f *testing.F) {
	for analyzer := VetAnalyzerLostCancel; analyzer <= VetAnalyzerCopyLocks; analyzer++ {
		f.Add(analyzer.String())
	}
	f.Add("future")
	f.Add("")
	f.Fuzz(func(t *testing.T, text string) {
		got, err := ParseVetAnalyzer(text)
		var want VetAnalyzer
		switch text {
		case "lostcancel":
			want = VetAnalyzerLostCancel
		case "waitgroup":
			want = VetAnalyzerWaitGroup
		case "copylocks":
			want = VetAnalyzerCopyLocks
		default:
			if got != VetAnalyzerUnknown || !errors.Is(err, core.ErrGoToolchainContract) {
				t.Fatalf("invalid label=(%v,%v),want zero,typed refusal", got, err)
			}
			return
		}
		if err != nil || got != want || got.Validate() != nil || got.String() != text {
			t.Fatalf("accepted label=(%v,%v),want%v,nil", got, err, want)
		}
	})
}
