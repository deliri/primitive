package runnercontrol

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/hostfacts"
	"github.com/deliri/primitive/v2026/jsonio"
	"github.com/deliri/primitive/v2026/testserial"
)

func TestGoVetStreamNativeRecordLayerTriad(t *testing.T) {
	t.Parallel()
	diagnostic := GoVetObservation{Unit: "u", Analyzer: "lostcancel", Position: "source.go:3:9", Message: "release cancel", Kind: GoVetObservationDiagnostic}
	analyzerError := GoVetObservation{Unit: "u", Analyzer: "lostcancel", Message: "analysis refused", Kind: GoVetObservationAnalyzerError}
	otherUnit, otherAnalyzer := diagnostic, diagnostic
	otherUnit.Unit, otherAnalyzer.Analyzer = "v", "copylocks"
	for _, tc := range []struct {
		name, input string
		want        []GoVetObservation
		refused     bool
	}{
		{name: "one native diagnostic", input: `{"u":{"lostcancel":[{"posn":"source.go:3:9","message":"release cancel"}]}}`, want: []GoVetObservation{diagnostic}},
		{name: "native analyzer failure", input: `{"u":{"lostcancel":{"error":"analysis refused"}}}`, want: []GoVetObservation{analyzerError}},
		{name: "native array multiplicity", input: `{"u":{"lostcancel":[{"posn":"source.go:3:9","message":"release cancel"},{"posn":"source.go:3:9","message":"release cancel"}]}}`, want: []GoVetObservation{diagnostic, diagnostic}},
		{name: "two native units retain distinct source labels", input: `{"u":{"lostcancel":[{"posn":"source.go:3:9","message":"release cancel"}]},"v":{"lostcancel":[{"posn":"source.go:3:9","message":"release cancel"}]}}`, want: []GoVetObservation{diagnostic, otherUnit}},
		{name: "two native analyzers retain distinct source labels", input: `{"u":{"lostcancel":[{"posn":"source.go:3:9","message":"release cancel"}],"copylocks":[{"posn":"source.go:3:9","message":"release cancel"}]}}`, want: []GoVetObservation{diagnostic, otherAnalyzer}},
		{name: "independent documents retain source order", input: `{"u":{"lostcancel":[{"posn":"source.go:3:9","message":"release cancel"}]}}{"u":{"lostcancel":{"error":"analysis refused"}}}`, want: []GoVetObservation{diagnostic, analyzerError}},
		{name: "native error before diagnostics retains source order", input: `{"u":{"lostcancel":{"error":"analysis refused"}}}{"u":{"lostcancel":[{"posn":"source.go:3:9","message":"release cancel"}]}}`, want: []GoVetObservation{analyzerError, diagnostic}},
		{name: "neutral empty source"},
		{name: "neutral empty native unit tree", input: `{}`},
		{name: "neutral unit without analyzers", input: `{"u":{}}`},
		{name: "neutral analyzer without diagnostics", input: `{"u":{"lostcancel":[]}}`},
		{name: "neutral multiple empty documents", input: `{}{"u":{}}{"u":{"lostcancel":[]}}`},
		{name: "null is not a unit tree", input: `null`, refused: true},
		{name: "array is not a unit tree", input: `[]`, refused: true},
		{name: "unit value is not an analyzer object", input: `{"u":[]}`, refused: true},
		{name: "analyzer value is not a diagnostic collection", input: `{"u":{"lostcancel":null}}`, refused: true},
		{name: "empty unit label is refused", input: `{"":{}}`, refused: true},
		{name: "empty analyzer label is refused", input: `{"u":{"":[]}}`, refused: true},
		{name: "numeric array item is refused", input: `{"u":{"lostcancel":[1]}}`, refused: true},
		{name: "missing position is refused", input: `{"u":{"lostcancel":[{"message":"release cancel"}]}}`, refused: true},
		{name: "missing message is refused", input: `{"u":{"lostcancel":[{"posn":"source.go:3:9"}]}}`, refused: true},
		{name: "null position is refused", input: `{"u":{"lostcancel":[{"posn":null,"message":"release cancel"}]}}`, refused: true},
		{name: "null message is refused", input: `{"u":{"lostcancel":[{"posn":"source.go:3:9","message":null}]}}`, refused: true},
		{name: "empty position is refused", input: `{"u":{"lostcancel":[{"posn":"","message":"release cancel"}]}}`, refused: true},
		{name: "wrong position type is refused", input: `{"u":{"lostcancel":[{"posn":3,"message":"release cancel"}]}}`, refused: true},
		{name: "wrong message type is refused", input: `{"u":{"lostcancel":[{"posn":"source.go:3:9","message":3}]}}`, refused: true},
		{name: "duplicate diagnostic field is refused", input: `{"u":{"lostcancel":[{"posn":"source.go:3:9","message":"first","message":"second"}]}}`, refused: true},
		{name: "duplicate analyzer is refused", input: `{"u":{"lostcancel":[],"lostcancel":[]}}`, refused: true},
		{name: "duplicate unit is refused", input: `{"u":{},"u":{}}`, refused: true},
		{name: "error object without native error is refused", input: `{"u":{"lostcancel":{"message":"analysis refused"}}}`, refused: true},
		{name: "null analyzer error is refused", input: `{"u":{"lostcancel":{"error":null}}}`, refused: true},
		{name: "wrong analyzer error type is refused", input: `{"u":{"lostcancel":{"error":3}}}`, refused: true},
		{name: "truncated diagnostic cannot emit a record", input: `{"u":{"lostcancel":[{"posn":"source.go:3:9","message":"release cancel"`, refused: true},
		{name: "complete diagnostic followed by truncated tree remains only a prefix", input: `{"u":{"lostcancel":[{"posn":"source.go:3:9","message":"release cancel"}]`, want: []GoVetObservation{diagnostic}, refused: true},
		{name: "valid document followed by foreign root remains only a prefix", input: `{"u":{"lostcancel":[{"posn":"source.go:3:9","message":"release cancel"}]}}[]`, want: []GoVetObservation{diagnostic}, refused: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var got []GoVetObservation
			err := ReadGoVetStream(t.Context(), GoVetStreamRequest{Source: strings.NewReader(tc.input), Visit: func(observation GoVetObservation) error {
				if err := observation.Validate(); err != nil {
					t.Fatalf("published native observation=%+v,%v", observation, err)
				}
				got = append(got, observation)
				return nil
			}})
			if tc.refused {
				if !errors.Is(err, core.ErrRunnerControlObservation) {
					t.Fatalf("native reader=%v,want observation refusal", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(got, tc.want) {
				t.Fatalf("native records=%+v,want%+v", got, tc.want)
			}
		})
	}
}

func TestGoVetStreamNativeFieldsAndExtents(t *testing.T) {
	t.Parallel()
	for _, size := range []int{0, 1, 4095, 4096, 4097, 65537} {
		t.Run(strconv.Itoa(size), func(t *testing.T) {
			t.Parallel()
			message := strings.Repeat("x", size)
			source := `{"u":{"native":[{"message":` + strconv.Quote(message) + `,"posn":"source:name.go:3:9","end":"source:name.go:3:11","category":"native category","url":"https://example.com/analysis","related":[{"posn":"other.go:1:1","message":"related"}],"suggested_fixes":[{"message":"fix","edits":[{"filename":"source:name.go","start":1,"end":2,"new":"replacement"}]}]}]}}`
			seen := 0
			err := ReadGoVetStream(t.Context(), GoVetStreamRequest{Source: strings.NewReader(source), Visit: func(got GoVetObservation) error {
				want := GoVetObservation{Kind: GoVetObservationDiagnostic, Unit: "u", Analyzer: "native", Position: "source:name.go:3:9", End: "source:name.go:3:11", Message: message}
				if got != want {
					t.Fatalf("native typed fields=%+v,want%+v", got, want)
				}
				seen++
				return nil
			}})
			if err != nil || seen != 1 {
				t.Fatalf("native field observations=%d,%v,want1,nil", seen, err)
			}
		})
	}
}

func TestGoVetStreamNativeContextAndVisitorCustody(t *testing.T) {
	t.Parallel()
	prefix := `{"u":{"native":[{"posn":"source.go:1:1","message":"one"},`
	for _, cancel := range []bool{false, true} {
		t.Run(fmt.Sprintf("cancel=%t", cancel), func(t *testing.T) {
			t.Parallel()
			ctx, stop := context.WithCancelCause(t.Context())
			defer stop(nil)
			seen := 0
			err := ReadGoVetStream(ctx, GoVetStreamRequest{Source: io.MultiReader(strings.NewReader(prefix), iotest.ErrReader(io.ErrClosedPipe)), Visit: func(GoVetObservation) error {
				seen++
				if cancel {
					stop(io.ErrNoProgress)
					return nil
				}
				return io.ErrNoProgress
			}})
			if seen != 1 || !errors.Is(err, io.ErrNoProgress) || errors.Is(err, io.ErrClosedPipe) || !errors.Is(err, core.ErrRunnerControlObservation) || cancel && !errors.Is(err, context.Canceled) {
				t.Fatalf("native prefix callback=%d,%v,want one visitor and exact stop/cancel cause before next source refusal", seen, err)
			}
		})
	}
	for _, prefix := range []string{"", `{}`, `{"u":{"a":[{"posn":"f:1:1","message":"m"}]}}`} {
		err := ReadGoVetStream(t.Context(), GoVetStreamRequest{Source: io.MultiReader(strings.NewReader(prefix), iotest.ErrReader(io.ErrClosedPipe)), Visit: func(GoVetObservation) error { return nil }})
		if !errors.Is(err, io.ErrClosedPipe) || !errors.Is(err, core.ErrRunnerControlObservation) {
			t.Fatalf("native source refusal=%v,want original and observation identities", err)
		}
	}
	for _, tc := range []struct {
		ctx     context.Context
		request GoVetStreamRequest
		want    error
	}{{ctx: nil, request: GoVetStreamRequest{Source: strings.NewReader("{}"), Visit: func(GoVetObservation) error { return nil }}, want: core.ErrNilContext}, {ctx: t.Context(), want: core.ErrPrimitiveContract}} {
		if err := ReadGoVetStream(tc.ctx, tc.request); !errors.Is(err, tc.want) {
			t.Fatalf("native absent custody=%v,want%v", err, tc.want)
		}
	}
}

func TestGoVetStreamNativePopulationDoesNotBecomeAnInventory(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	for _, count := range []int{64, 16385} {
		t.Run(strconv.Itoa(count), func(t *testing.T) {
			input := []byte(`{"u":{"a":[` + strings.TrimSuffix(strings.Repeat(`{"posn":"f:1:1","message":"m"},`, count), ",") + `]}}`)
			before, err := hostfacts.ObserveCollectedGoHeap(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			seen := 0
			err = ReadGoVetStream(t.Context(), GoVetStreamRequest{Source: bytes.NewReader(input), Visit: func(GoVetObservation) error {
				seen++
				if seen == 1 {
					after, err := hostfacts.ObserveCollectedGoHeap(t.Context())
					if err != nil {
						return err
					}
					growth := int64(after.Uint64()) - int64(before.Uint64())
					t.Logf("native vet records=%d first-record retained bytes=%d", count, growth)
					if growth > 256<<10 {
						t.Fatalf("native vet retained %d bytes of record population", growth)
					}
				}
				return nil
			}})
			if err != nil || seen != count {
				t.Fatalf("native vet records=%d,%v,want%d,nil", seen, err, count)
			}
			runtime.KeepAlive(input)
		})
	}
}

func TestGoVetObservationClosedContracts(t *testing.T) {
	t.Parallel()
	for _, kind := range []GoVetObservationKind{GoVetObservationUnknown, GoVetObservationDiagnostic, GoVetObservationAnalyzerError, 3, 255} {
		want := kind == GoVetObservationDiagnostic || kind == GoVetObservationAnalyzerError
		if kind.IsValid() != want || (kind.Validate() == nil) != want || kind.String() == "" {
			t.Fatalf("native vet kind=%d valid=%v,error=%v,text=%q,want admitted=%v", kind, kind.IsValid(), kind.Validate(), kind.String(), want)
		}
		if !want && !errors.Is(kind.Validate(), core.ErrRunnerControlObservation) {
			t.Fatalf("native vet kind%d error=%v,want observation identity", kind, kind.Validate())
		}
		kind.OffWireEnum()
	}
	for _, tc := range []struct {
		name string
		in   GoVetObservation
		want bool
	}{
		{name: "native fileless diagnostic", in: GoVetObservation{Kind: GoVetObservationDiagnostic, Unit: "u", Analyzer: "native", Position: "-"}, want: true},
		{name: "native empty diagnostic message", in: GoVetObservation{Kind: GoVetObservationDiagnostic, Unit: "u", Analyzer: "native", Position: "f:1:1"}, want: true},
		{name: "native empty analyzer error message", in: GoVetObservation{Kind: GoVetObservationAnalyzerError, Unit: "u", Analyzer: "native"}, want: true},
		{name: "unknown observation kind", in: GoVetObservation{Unit: "u", Analyzer: "native", Position: "f:1:1"}},
		{name: "error cannot borrow diagnostic position", in: GoVetObservation{Kind: GoVetObservationAnalyzerError, Unit: "u", Analyzer: "native", Position: "f:1:1"}},
		{name: "error cannot borrow diagnostic end", in: GoVetObservation{Kind: GoVetObservationAnalyzerError, Unit: "u", Analyzer: "native", End: "f:1:2"}},
		{name: "missing unit label", in: GoVetObservation{Kind: GoVetObservationAnalyzerError, Analyzer: "native"}},
		{name: "missing analyzer label", in: GoVetObservation{Kind: GoVetObservationAnalyzerError, Unit: "u"}},
		{name: "invalid UTF8 message", in: GoVetObservation{Kind: GoVetObservationAnalyzerError, Unit: "u", Analyzer: "native", Message: "\xff"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.in.Validate()
			if tc.want && err != nil || !tc.want && !errors.Is(err, core.ErrRunnerControlObservation) {
				t.Fatalf("native observation=%+v,%v,want admitted=%v", tc.in, err, tc.want)
			}
		})
	}
}

type goVetFixtureAnalyzers struct {
	Native []goVetDiagnosticWire `json:"native"`
}
type goVetFixtureDocument struct {
	Unit goVetFixtureAnalyzers `json:"u"`
}
type goVetErrorFixtureAnalyzers struct {
	Native goVetAnalyzerErrorWire `json:"native"`
}
type goVetErrorFixtureDocument struct {
	Unit goVetErrorFixtureAnalyzers `json:"u"`
}

func FuzzGoVetStreamNativeProjectionConservation(f *testing.F) {
	for mode := uint8(0); mode < 8; mode++ {
		f.Add(mode, uint8(1), []byte("native compiler diagnostic"))
	}
	f.Fuzz(func(t *testing.T, mode, extent uint8, text []byte) {
		message := string(bytes.ToValidUTF8(text, []byte("\ufffd")))
		position, end := "source.go:3:9", "source.go:3:10"
		count := int(extent % 17)
		if mode%8 == 4 || mode%8 == 5 {
			count++
		}
		wire := goVetDiagnosticWire{Position: &position, End: end, Message: &message}
		if mode%8 == 4 {
			wire.Position = nil
		}
		if mode%8 == 5 {
			wire.Message = nil
		}
		document := goVetFixtureDocument{Unit: goVetFixtureAnalyzers{Native: make([]goVetDiagnosticWire, count)}}
		for index := range document.Unit.Native {
			document.Unit.Native[index] = wire
		}
		policy := jsonio.OwnedCodecPolicy{Root: jsonio.RootDeclaredFields, PointerNulls: jsonio.PointerNullPermitted}
		encoded, err := jsonio.MarshalProjection(document, policy)
		if err != nil {
			t.Fatal(err)
		}
		errorDocument := goVetErrorFixtureDocument{Unit: goVetErrorFixtureAnalyzers{Native: goVetAnalyzerErrorWire{Error: &message}}}
		errorEncoded, err := jsonio.MarshalProjection(errorDocument, policy)
		if err != nil {
			t.Fatal(err)
		}
		wantCount, wantErrorRecord, wantRefusal := count, false, false
		switch mode % 8 {
		case 1:
			encoded, wantCount, wantErrorRecord = errorEncoded, 1, true
		case 2:
			encoded = append(encoded, errorEncoded...)
			wantCount, wantErrorRecord = count+1, true
		case 3:
			encoded, err = jsonio.MarshalProjection(struct{}{}, policy)
			if err != nil {
				t.Fatal(err)
			}
			wantCount = 0
		case 4, 5:
			wantCount, wantRefusal = 0, true
		case 6:
			encoded, wantCount, wantRefusal = []byte(`{"u":{"native":null}}`), 0, true
		case 7:
			encoded, wantRefusal = encoded[:len(encoded)-1], true
		}
		seen := 0
		err = ReadGoVetStream(t.Context(), GoVetStreamRequest{Source: bytes.NewReader(encoded), Visit: func(got GoVetObservation) error {
			want := GoVetObservation{Kind: GoVetObservationDiagnostic, Unit: "u", Analyzer: "native", Position: position, End: end, Message: message}
			if wantErrorRecord && seen == wantCount-1 {
				want.Kind, want.Position, want.End = GoVetObservationAnalyzerError, "", ""
			}
			if got != want || seen >= wantCount || got.Validate() != nil {
				t.Fatalf("native record%d=%+v,want%+v,total%d", seen, got, want, wantCount)
			}
			seen++
			return nil
		}})
		if seen != wantCount || wantRefusal && !errors.Is(err, core.ErrRunnerControlObservation) || !wantRefusal && err != nil {
			t.Fatalf("native mode%d records=%d,%v,want%d and refusal=%v", mode%8, seen, err, wantCount, wantRefusal)
		}
	})
}
