package temporal

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
)

type (
	temporalSealedValue[T any]      struct{}
	temporalIngressRequest[T any]   struct{}
	temporalPersistenceFact[T any]  struct{}
	temporalCapabilityIntent[T any] struct{}
	temporalDefinitionFact[T any]   struct{}
)

type temporalContractInventory struct {
	Instant           temporalSealedValue[Instant]
	Duration          temporalSealedValue[Duration]
	AggregateDuration temporalSealedValue[AggregateDuration]
	Observation       temporalSealedValue[Observation]
	Interval          temporalSealedValue[Interval]
	NumericInstant    temporalPersistenceFact[NumericInstant]
	NumericDuration   temporalPersistenceFact[NumericDuration]
	IntervalRequest   temporalIngressRequest[IntervalRequest]
	IntervalBounds    temporalPersistenceFact[IntervalBounds]
	TimeoutRequest    temporalCapabilityIntent[TimeoutRequest]
	DeadlineRequest   temporalCapabilityIntent[DeadlineRequest]
	WaitRequest       temporalCapabilityIntent[WaitRequest]
	TickerRequest     temporalCapabilityIntent[TickerRequest]
	Ticker            temporalCapabilityIntent[Ticker]
	ContextResult     temporalCapabilityIntent[contextConstruction]
	PrecisionFact     temporalDefinitionFact[precisionFact]
}

var _ = temporalContractInventory{}

// These interfaces and function assignments are the compiler-owned half of
// the reviewed public-surface contract. The AST ratchet below rejects added or
// removed names; these witnesses reject signature drift under an existing name.
type temporalInstantSignature interface {
	Validate() error
	IsSet() bool
	Nanoseconds() (int64, error)
	Time() (time.Time, error)
	RFC3339() (string, error)
	RFC3339Nano() (string, error)
	Add(Duration) (Instant, error)
	Subtract(Duration) (Instant, error)
	Since(Instant) (Duration, error)
	Compare(Instant) (core.Comparison, error)
	Truncate(Precision) (Instant, error)
	MarshalJSON() ([]byte, error)
}

type temporalDurationSignature interface {
	Validate() error
	Nanoseconds() int64
	Stdlib() (time.Duration, error)
	IsZero() bool
	Add(Duration) (Duration, error)
	Subtract(Duration) (Duration, error)
	Multiply(uint64) (Duration, error)
	Compare(Duration) (core.Comparison, error)
	Aggregate() (AggregateDuration, error)
	MarshalJSON() ([]byte, error)
}

type temporalAggregateDurationSignature interface {
	Validate() error
	IsZero() bool
	Decimal() string
	Add(AggregateDuration) (AggregateDuration, error)
	AddDuration(Duration) (AggregateDuration, error)
	Multiply(uint64) (AggregateDuration, error)
	Compare(AggregateDuration) core.Comparison
	Duration() (Duration, error)
	MarshalJSON() ([]byte, error)
}

type temporalObservationSignature interface {
	Validate() error
	Instant() (Instant, error)
	WithWall(Instant) (Observation, error)
	Since(Observation) (Duration, error)
}

type temporalIntervalSignature interface {
	Validate() error
	Start() (Instant, error)
	End() (Instant, error)
	Elapsed() (Duration, error)
	Bounds() (IntervalBounds, error)
}

type temporalNumericInstantSignature interface {
	Validate() error
	IsSet() bool
	Instant() (Instant, error)
	MarshalJSON() ([]byte, error)
}

type temporalNumericDurationSignature interface {
	Validate() error
	IsZero() bool
	Duration() Duration
	MarshalJSON() ([]byte, error)
}

type temporalPrecisionSignature interface {
	Validate() error
	IsValid() bool
	OffWireEnum()
	String() string
}

var (
	_ temporalInstantSignature                 = Instant{}
	_ interface{ UnmarshalJSON([]byte) error } = (*Instant)(nil)
	_ temporalDurationSignature                = Duration{}
	_ interface{ UnmarshalJSON([]byte) error } = (*Duration)(nil)
	_ temporalAggregateDurationSignature       = AggregateDuration{}
	_ interface{ UnmarshalJSON([]byte) error } = (*AggregateDuration)(nil)
	_ temporalNumericInstantSignature          = NumericInstant{}
	_ interface{ UnmarshalJSON([]byte) error } = (*NumericInstant)(nil)
	_ temporalNumericDurationSignature         = NumericDuration{}
	_ interface{ UnmarshalJSON([]byte) error } = (*NumericDuration)(nil)
	_ temporalObservationSignature             = Observation{}
	_ temporalIntervalSignature                = Interval{}
	_ temporalPrecisionSignature               = PrecisionUnknown

	_ interface{ Validate() error } = IntervalRequest{}
	_ interface{ Validate() error } = IntervalBounds{}
	_ interface{ Validate() error } = TimeoutRequest{}
	_ interface{ Validate() error } = DeadlineRequest{}
	_ interface{ Validate() error } = WaitRequest{}
	_ interface{ Validate() error } = TickerRequest{}
	_ interface{ Validate() error } = (*Ticker)(nil)

	_ = IntervalRequest{Observation{}, Observation{}}
	_ = IntervalBounds{Instant{}, Instant{}}
	_ = TimeoutRequest{context.Background(), Duration{}}
	_ = DeadlineRequest{context.Background(), Instant{}}
	_ = WaitRequest{context.Background(), Duration{}}
	_ = TickerRequest{Duration{}}

	_ func(time.Time) (Instant, error)                                   = NewInstant
	_ func(string) (Instant, error)                                      = ParseRFC3339
	_ func(int64) Instant                                                = InstantFromNanoseconds
	_ func(time.Duration) (Duration, error)                              = NewDuration
	_ func(int64) (Duration, error)                                      = DurationFromNanoseconds
	_ func(uint64) (Duration, error)                                     = DurationFromMicroseconds
	_ func(uint64) (Duration, error)                                     = DurationFromMilliseconds
	_ func(uint64) (Duration, error)                                     = DurationFromSeconds
	_ func(uint64) (Duration, error)                                     = DurationFromMinutes
	_ func(uint64) (Duration, error)                                     = DurationFromHours
	_ func(uint64) (Duration, error)                                     = DurationFromDays
	_ func(uint64) AggregateDuration                                     = AggregateDurationFromNanoseconds
	_ func(Duration) (AggregateDuration, error)                          = AggregateDurationFromDuration
	_ func(string) (AggregateDuration, error)                            = ParseAggregateDuration
	_ func(Instant) (NumericInstant, error)                              = NewNumericInstant
	_ func(Duration) (NumericDuration, error)                            = NewNumericDuration
	_ func() (Observation, error)                                        = Observe
	_ func(time.Time) (Observation, error)                               = NewObservation
	_ func(IntervalRequest) (Interval, error)                            = NewInterval
	_ func(IntervalBounds) (Interval, error)                             = IntervalFromBounds
	_ func(TimeoutRequest) (context.Context, context.CancelFunc, error)  = WithTimeout
	_ func(DeadlineRequest) (context.Context, context.CancelFunc, error) = WithDeadline
	_ func(WaitRequest) error                                            = Wait
	_ func(TickerRequest) (*Ticker, error)                               = OpenTicker
)

type temporalArchitecture struct {
	structs          []string
	surface          []string
	imports          []string
	aliases          []string
	mapFiles         []string
	goStatementFiles []string
	channelMakeFiles []string
}

func TestTemporalProductionStructsHaveCompilerVisibleDataFlowRoles(t *testing.T) {
	t.Parallel()

	got, gotErr := scanTemporalArchitecture(".")
	if gotErr != nil {
		t.Fatalf("scanTemporalArchitecture() error = %v, want nil", gotErr)
	}
	want := []string{
		"AggregateDuration",
		"DeadlineRequest",
		"Duration",
		"Instant",
		"Interval",
		"IntervalBounds",
		"IntervalRequest",
		"NumericDuration",
		"NumericInstant",
		"Observation",
		"Ticker",
		"TickerRequest",
		"TimeoutRequest",
		"WaitRequest",
		"contextConstruction",
		"precisionFact",
	}
	if !slices.Equal(got.structs, want) {
		t.Fatalf("Temporal production structs = %q, want classified %q", got.structs, want)
	}
}

func TestTemporalPublicSurfaceMatchesReviewedContract(t *testing.T) {
	t.Parallel()

	got, gotErr := scanTemporalArchitecture(".")
	if gotErr != nil {
		t.Fatalf("scanTemporalArchitecture() error = %v, want nil", gotErr)
	}
	want := []string{
		"const AggregateDurationCanonicalJSONMaximumBytes",
		"const AggregateDurationJSONMaximumBytes",
		"const AggregateDurationMaximumDecimalDigits",
		"const DurationCanonicalJSONMaximumBytes",
		"const DurationJSONMaximumBytes",
		"const DurationMaximumNanoseconds",
		"const InstantCanonicalJSONMaximumBytes",
		"const InstantJSONMaximumBytes",
		"const NanosecondsPerDay",
		"const NanosecondsPerHour",
		"const NanosecondsPerMicrosecond",
		"const NanosecondsPerMillisecond",
		"const NanosecondsPerMinute",
		"const NanosecondsPerSecond",
		"const NumericDurationCanonicalJSONMaximumBytes",
		"const NumericInstantCanonicalJSONMaximumBytes",
		"const PrecisionMicrosecond",
		"const PrecisionMillisecond",
		"const PrecisionNanosecond",
		"const PrecisionSecond",
		"const PrecisionUnknown",
		"const RFC3339MaximumTextBytes",
		"const RFC3339MinimumTextBytes",
		"const TemporalJSONDocumentSlackBytes",
		"func AggregateDurationFromDuration",
		"func AggregateDurationFromNanoseconds",
		"func DurationFromDays",
		"func DurationFromHours",
		"func DurationFromMicroseconds",
		"func DurationFromMilliseconds",
		"func DurationFromMinutes",
		"func DurationFromNanoseconds",
		"func DurationFromSeconds",
		"func InstantFromNanoseconds",
		"func IntervalFromBounds",
		"func NewDuration",
		"func NewInstant",
		"func NewInterval",
		"func NewNumericDuration",
		"func NewNumericInstant",
		"func NewObservation",
		"func Observe",
		"func OpenTicker",
		"func ParseAggregateDuration",
		"func ParseDuration",
		"func ParseRFC3339",
		"func Wait",
		"func WithDeadline",
		"func WithTimeout",
		"method AggregateDuration.Add",
		"method AggregateDuration.AddDuration",
		"method AggregateDuration.Compare",
		"method AggregateDuration.Decimal",
		"method AggregateDuration.Duration",
		"method AggregateDuration.IsZero",
		"method AggregateDuration.MarshalJSON",
		"method AggregateDuration.Multiply",
		"method AggregateDuration.UnmarshalJSON",
		"method AggregateDuration.Validate",
		"method DeadlineRequest.Validate",
		"method Duration.Add",
		"method Duration.Aggregate",
		"method Duration.Compare",
		"method Duration.IsZero",
		"method Duration.MarshalJSON",
		"method Duration.Multiply",
		"method Duration.Nanoseconds",
		"method Duration.Stdlib",
		"method Duration.Subtract",
		"method Duration.UnmarshalJSON",
		"method Duration.Validate",
		"method Instant.Add",
		"method Instant.Compare",
		"method Instant.IsSet",
		"method Instant.MarshalJSON",
		"method Instant.Nanoseconds",
		"method Instant.RFC3339",
		"method Instant.RFC3339Nano",
		"method Instant.Since",
		"method Instant.Subtract",
		"method Instant.Time",
		"method Instant.Truncate",
		"method Instant.UnmarshalJSON",
		"method Instant.Validate",
		"method Interval.Bounds",
		"method Interval.Elapsed",
		"method Interval.End",
		"method Interval.Start",
		"method Interval.Validate",
		"method IntervalBounds.Validate",
		"method IntervalRequest.Validate",
		"method NumericDuration.Duration",
		"method NumericDuration.IsZero",
		"method NumericDuration.MarshalJSON",
		"method NumericDuration.UnmarshalJSON",
		"method NumericDuration.Validate",
		"method NumericInstant.Instant",
		"method NumericInstant.IsSet",
		"method NumericInstant.MarshalJSON",
		"method NumericInstant.UnmarshalJSON",
		"method NumericInstant.Validate",
		"method Observation.Instant",
		"method Observation.Since",
		"method Observation.Validate",
		"method Observation.WithWall",
		"method Precision.IsValid",
		"method Precision.OffWireEnum",
		"method Precision.String",
		"method Precision.Validate",
		"method TickerRequest.Validate",
		"method Ticker.Stop",
		"method Ticker.Ticks",
		"method Ticker.Validate",
		"method TimeoutRequest.Validate",
		"method WaitRequest.Validate",
		"type AggregateDuration",
		"type DeadlineRequest",
		"type Duration",
		"type Instant",
		"type Interval",
		"type IntervalBounds",
		"type IntervalRequest",
		"type NumericDuration",
		"type NumericInstant",
		"type Observation",
		"type Precision",
		"type TickerRequest",
		"type Ticker",
		"type TimeoutRequest",
		"type WaitRequest",
	}
	slices.Sort(want)
	if !slices.Equal(got.surface, want) {
		t.Fatalf("Temporal public surface = %q, want %q", got.surface, want)
	}
	if len(got.aliases) != 0 {
		t.Fatalf("Temporal production aliases = %q, want none", got.aliases)
	}
}

func TestTemporalProductionStaysOnGoContextAndTimePrimitives(t *testing.T) {
	t.Parallel()

	got, gotErr := scanTemporalArchitecture(".")
	if gotErr != nil {
		t.Fatalf("scanTemporalArchitecture() error = %v, want nil", gotErr)
	}
	wantImports := []string{
		"bytes",
		"context",
		"encoding/json/v2",
		"errors",
		"github.com/deliri/primitive/v2026/contextstate",
		"github.com/deliri/primitive/v2026/core",
		"math",
		"math/bits",
		"strconv",
		"time",
	}
	if !slices.Equal(got.imports, wantImports) {
		t.Fatalf("Temporal production imports = %q, want %q", got.imports, wantImports)
	}
	if len(got.mapFiles) != 0 {
		t.Fatalf("Temporal production map types = %q, want none", got.mapFiles)
	}
	if len(got.goStatementFiles) != 0 {
		t.Fatalf("Temporal production go statements = %q, want none", got.goStatementFiles)
	}
	if len(got.channelMakeFiles) != 0 {
		t.Fatalf("Temporal production channel allocations = %q, want none", got.channelMakeFiles)
	}
}

func TestTemporalArchitectureMatcherProvesForbiddenShapes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name                  string
		source                string
		wantStructs           []string
		wantSurface           []string
		wantAliases           []string
		wantMap               bool
		wantGoStatement       bool
		wantChannelAllocation bool
	}{
		{
			name:        "named request struct is inventoried and exported",
			source:      "package temporal\ntype Request struct{ Value int }\n",
			wantStructs: []string{"Request"},
			wantSurface: []string{"type Request"},
		},
		{
			name:        "alias is detected without becoming a struct",
			source:      "package temporal\ntype Value uint8\ntype Alias = Value\n",
			wantSurface: []string{"type Alias", "type Value"},
			wantAliases: []string{"Alias"},
		},
		{
			name:    "map-backed world model is detected",
			source:  "package temporal\nvar values map[string]int\n",
			wantMap: true,
		},
		{
			name:            "production goroutine is detected",
			source:          "package temporal\nfunc Start() { go func() {}() }\n",
			wantSurface:     []string{"func Start"},
			wantGoStatement: true,
		},
		{
			name:                  "private channel allocation is detected",
			source:                "package temporal\nfunc start() { _ = make(chan struct{}) }\n",
			wantChannelAllocation: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := scanTemporalSource("synthetic.go", []byte(tc.source))
			if gotErr != nil {
				t.Fatalf("scanTemporalSource() error = %v, want nil", gotErr)
			}
			if !slices.Equal(got.structs, tc.wantStructs) ||
				!slices.Equal(got.surface, tc.wantSurface) ||
				!slices.Equal(got.aliases, tc.wantAliases) ||
				(len(got.mapFiles) != 0) != tc.wantMap ||
				(len(got.goStatementFiles) != 0) != tc.wantGoStatement ||
				(len(got.channelMakeFiles) != 0) != tc.wantChannelAllocation {
				t.Fatalf(
					"synthetic scan = structs:%q surface:%q aliases:%q map:%q go:%q channel:%q",
					got.structs,
					got.surface,
					got.aliases,
					got.mapFiles,
					got.goStatementFiles,
					got.channelMakeFiles,
				)
			}
		})
	}
}

func scanTemporalArchitecture(directory string) (temporalArchitecture, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return temporalArchitecture{}, err
	}
	var result temporalArchitecture
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") ||
			strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		data, readErr := os.ReadFile(filepath.Join(directory, entry.Name()))
		if readErr != nil {
			return temporalArchitecture{}, readErr
		}
		scanned, scanErr := scanTemporalSource(entry.Name(), data)
		if scanErr != nil {
			return temporalArchitecture{}, scanErr
		}
		result.merge(scanned)
	}
	result.sort()
	return result, nil
}

func scanTemporalSource(name string, data []byte) (temporalArchitecture, error) {
	file, err := parser.ParseFile(token.NewFileSet(), name, data, 0)
	if err != nil {
		return temporalArchitecture{}, err
	}
	var result temporalArchitecture
	scanTemporalDeclarations(file, &result)
	scanTemporalImports(file, &result)
	scanTemporalForbiddenShapes(name, file, &result)
	result.sort()
	return result, nil
}

func scanTemporalDeclarations(file *ast.File, result *temporalArchitecture) {
	for _, declaration := range file.Decls {
		switch value := declaration.(type) {
		case *ast.GenDecl:
			scanTemporalGeneralDeclaration(value, result)
		case *ast.FuncDecl:
			if value.Name.IsExported() {
				result.surface = append(result.surface, temporalFunctionSurface(value))
			}
		}
	}
}

func scanTemporalGeneralDeclaration(declaration *ast.GenDecl, result *temporalArchitecture) {
	for _, specification := range declaration.Specs {
		typeSpecification, ok := specification.(*ast.TypeSpec)
		if ok {
			if _, isStruct := typeSpecification.Type.(*ast.StructType); isStruct {
				result.structs = append(result.structs, typeSpecification.Name.Name)
			}
			if typeSpecification.Assign.IsValid() {
				result.aliases = append(result.aliases, typeSpecification.Name.Name)
			}
			if typeSpecification.Name.IsExported() {
				result.surface = append(result.surface, "type "+typeSpecification.Name.Name)
			}
			continue
		}
		valueSpecification, ok := specification.(*ast.ValueSpec)
		if !ok {
			continue
		}
		kind := declaration.Tok.String()
		for _, identifier := range valueSpecification.Names {
			if identifier.IsExported() {
				result.surface = append(result.surface, kind+" "+identifier.Name)
			}
		}
	}
}

func temporalFunctionSurface(function *ast.FuncDecl) string {
	if function.Recv == nil {
		return "func " + function.Name.Name
	}
	receiver := function.Recv.List[0].Type
	if star, ok := receiver.(*ast.StarExpr); ok {
		receiver = star.X
	}
	identifier, _ := receiver.(*ast.Ident)
	return "method " + identifier.Name + "." + function.Name.Name
}

func scanTemporalImports(file *ast.File, result *temporalArchitecture) {
	for _, specification := range file.Imports {
		path, err := strconv.Unquote(specification.Path.Value)
		if err != nil {
			continue
		}
		result.imports = append(result.imports, path)
	}
}

func scanTemporalForbiddenShapes(
	name string,
	file *ast.File,
	result *temporalArchitecture,
) {
	ast.Inspect(file, func(node ast.Node) bool {
		switch value := node.(type) {
		case *ast.MapType:
			result.mapFiles = append(result.mapFiles, name)
		case *ast.GoStmt:
			result.goStatementFiles = append(result.goStatementFiles, name)
		case *ast.CallExpr:
			if temporalCallAllocatesChannel(value) {
				result.channelMakeFiles = append(result.channelMakeFiles, name)
			}
		}
		return true
	})
}

func temporalCallAllocatesChannel(call *ast.CallExpr) bool {
	identifier, ok := call.Fun.(*ast.Ident)
	if !ok || identifier.Name != "make" || len(call.Args) == 0 {
		return false
	}
	_, ok = call.Args[0].(*ast.ChanType)
	return ok
}

func (a *temporalArchitecture) merge(other temporalArchitecture) {
	a.structs = append(a.structs, other.structs...)
	a.surface = append(a.surface, other.surface...)
	a.imports = append(a.imports, other.imports...)
	a.aliases = append(a.aliases, other.aliases...)
	a.mapFiles = append(a.mapFiles, other.mapFiles...)
	a.goStatementFiles = append(a.goStatementFiles, other.goStatementFiles...)
	a.channelMakeFiles = append(a.channelMakeFiles, other.channelMakeFiles...)
}

func (a *temporalArchitecture) sort() {
	slices.Sort(a.structs)
	slices.Sort(a.surface)
	slices.Sort(a.imports)
	a.imports = slices.Compact(a.imports)
	slices.Sort(a.aliases)
	slices.Sort(a.mapFiles)
	a.mapFiles = slices.Compact(a.mapFiles)
	slices.Sort(a.goStatementFiles)
	a.goStatementFiles = slices.Compact(a.goStatementFiles)
	slices.Sort(a.channelMakeFiles)
	a.channelMakeFiles = slices.Compact(a.channelMakeFiles)
}
