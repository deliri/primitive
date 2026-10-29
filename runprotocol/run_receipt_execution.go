package runprotocol

import (
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/jsonio"
	"github.com/deliri/primitive/v2026/process"
	"github.com/deliri/primitive/v2026/temporal"
	"slices"
	"strconv"
)

// RunKind records the admitted native workload, not a guess from a test name.
type RunKind uint8

const (
	RunKindUnknown RunKind = iota
	RunKindTest
	RunKindBenchmark
	RunKindFuzz
)

type RunSourceKind uint8

const (
	RunSourceUnknown RunSourceKind = iota
	RunSourceCommit
	RunSourceSnapshot
)

type RunScopeKind uint8

const (
	RunScopeUnknown RunScopeKind = iota
	RunScopeFile
	RunScopePackage
	RunScopeSelected
	RunScopeProject
)

type RunFactKind uint8

const (
	RunFactUnknown RunFactKind = iota
	RunFactInvocation
	RunFactExecution
	RunFactResult
)

type RunResultKind uint8

const (
	RunResultUnknown RunResultKind = iota
	RunResultPackage
	RunResultTest
	RunResultBenchmark
	RunResultFuzz
)

const runReceiptKindInvalidDiagnostic = "run receipt kind is invalid"

type RunSource struct {
	Kind       RunSourceKind      `json:"kind"`
	Repository RepositoryIdentity `json:"repository"`
	Commit     core.BuildCommit   `json:"commit"`
	Captured   *RunSourceCapture  `json:"captured,omitempty"`
}
type RunSourceCapture struct {
	Digest core.SHA256Digest `json:"digest"`
	Files  uint64            `json:"files"`
	Bytes  core.ByteLength   `json:"bytes"`
}

func (r RunSourceCapture) Validate() error {
	if r.Files == 0 {
		return core.ErrRunProtocolContract
	}
	return contractJoin(r.Digest.Validate(), r.Bytes.Validate())
}
func (r RunSource) Validate() error {
	if err := contractJoin(r.Kind.Validate(), r.Repository.Validate(), r.Commit.Validate()); err != nil {
		return err
	}
	if r.Captured != nil {
		return r.Captured.Validate()
	}
	return nil
}

type RunScope struct {
	Kind    RunScopeKind        `json:"kind"`
	Targets []core.RelativePath `json:"targets"`
}

func (r RunScope) Validate() error {
	if err := r.Kind.Validate(); err != nil {
		return err
	}
	if len(r.Targets) == 0 || r.Kind != RunScopeSelected && len(r.Targets) != 1 {
		return core.ErrRunProtocolContract
	}
	for i, p := range r.Targets {
		if err := p.Validate(); err != nil {
			return contractError(err)
		}
		if slices.Contains(r.Targets[:i], p) {
			return conflictError(nil)
		}
	}
	return nil
}

type RunWorker struct {
	Name              Identifier        `json:"name"`
	Platform          core.Platform     `json:"platform"`
	Executable        core.SHA256Digest `json:"executable"`
	ObservedAt        temporal.Instant  `json:"observed_at"`
	LogicalCPUs       uint32            `json:"logical_cpus"`
	TotalMemoryBytes  core.ByteLength   `json:"total_memory_bytes"`
	ScratchTotalBytes core.ByteLength   `json:"scratch_total_bytes"`
}

func (r RunWorker) Validate() error {
	if r.LogicalCPUs == 0 || r.TotalMemoryBytes.Uint64() == 0 || r.ScratchTotalBytes.Uint64() == 0 {
		return core.ErrRunProtocolContract
	}
	return contractJoin(r.Name.Validate(), r.Platform.Validate(), r.Executable.Validate(), r.ObservedAt.Validate(), r.TotalMemoryBytes.Validate(), r.ScratchTotalBytes.Validate())
}

type RunBudget struct {
	CPU          uint16            `json:"cpu"`
	MemoryBytes  core.ByteLength   `json:"memory_bytes"`
	ScratchBytes core.ByteLength   `json:"scratch_bytes"`
	Timeout      temporal.Duration `json:"timeout"`
}

func (r RunBudget) Validate() error {
	if r.CPU == 0 || r.MemoryBytes.Uint64() == 0 || r.ScratchBytes.Uint64() == 0 || r.Timeout.IsZero() {
		return core.ErrRunProtocolContract
	}
	return contractJoin(r.MemoryBytes.Validate(), r.ScratchBytes.Validate(), r.Timeout.Validate())
}

type RunTiming struct {
	CoordinatorBeforeLaunch *temporal.Duration `json:"coordinator_before_launch,omitempty"`
	Preparation             *temporal.Duration `json:"preparation,omitempty"`
	Admission               *temporal.Duration `json:"admission,omitempty"`
	BuildAndRun             *temporal.Duration `json:"build_and_run,omitempty"`
	BeforePackageStarts     *temporal.Duration `json:"before_package_starts,omitempty"`
}

func (r RunTiming) Validate() error {
	for _, d := range [...]*temporal.Duration{r.CoordinatorBeforeLaunch, r.Preparation, r.Admission, r.BuildAndRun, r.BeforePackageStarts} {
		if d != nil {
			if err := d.Validate(); err != nil {
				return contractError(err)
			}
		}
	}
	if r.BuildAndRun != nil && r.Preparation == nil || r.Admission != nil && (r.Preparation == nil || r.Admission.Nanoseconds() > r.Preparation.Nanoseconds()) || r.BeforePackageStarts != nil && (r.BuildAndRun == nil || r.BeforePackageStarts.Nanoseconds() > r.BuildAndRun.Nanoseconds()) {
		return conflictError(nil)
	}
	return nil
}

type RunCounts struct {
	StartedCount uint64 `json:"started"`
	Passed       uint64 `json:"passed"`
	Failed       uint64 `json:"failed"`
	Skipped      uint64 `json:"skipped"`
}

func (r RunCounts) Validate() error {
	remaining := r.StartedCount
	for _, n := range [...]uint64{r.Passed, r.Failed, r.Skipped} {
		if n > remaining {
			return conflictError(nil)
		}
		remaining -= n
	}
	return nil
}

type RunBenchmarkCounts struct {
	Rows     uint64 `json:"rows"`
	Measured uint64 `json:"measured"`
}

func (r RunBenchmarkCounts) Validate() error {
	if r.Measured > r.Rows {
		return conflictError(nil)
	}
	return nil
}

type RunFuzzCounts struct {
	Executions     *uint64 `json:"executions,omitempty"`
	NewInteresting *uint64 `json:"new_interesting,omitempty"`
}

func (r RunFuzzCounts) Validate() error {
	if r.Executions == nil && r.NewInteresting == nil {
		return core.ErrRunProtocolContract
	}
	return nil
}

type RunMemoryObservation struct {
	SampledPeak core.ByteLength   `json:"sampled_peak_bytes"`
	Samples     uint32            `json:"samples"`
	Interval    temporal.Duration `json:"interval"`
	Failure     *Text             `json:"failure,omitempty"`
}

func (r RunMemoryObservation) Validate() error {
	if r.Interval.IsZero() || r.Samples == 0 && r.SampledPeak.Uint64() != 0 || r.Samples == 0 && r.Failure == nil {
		return core.ErrRunProtocolContract
	}
	if r.Failure != nil {
		if err := r.Failure.Validate(); err != nil {
			return err
		}
	}
	return contractJoin(r.SampledPeak.Validate(), r.Interval.Validate())
}

type RunSummary struct {
	StreamDecoded bool                  `json:"stream_decoded"`
	Tests         RunCounts             `json:"tests"`
	Packages      RunCounts             `json:"packages"`
	BuildFailed   bool                  `json:"build_failed"`
	ExitCode      *int64                `json:"exit_code,omitempty"`
	Benchmarks    *RunBenchmarkCounts   `json:"benchmarks,omitempty"`
	Fuzz          *RunFuzzCounts        `json:"fuzz,omitempty"`
	Memory        *RunMemoryObservation `json:"memory,omitempty"`
	Failure       *Text                 `json:"failure,omitempty"`
}

func (r RunSummary) Validate() error {
	if err := contractJoin(r.Tests.Validate(), r.Packages.Validate()); err != nil {
		return err
	}
	if r.ExitCode != nil && (*r.ExitCode < core.ProcessExitCodeSignaled || *r.ExitCode > core.ProcessExitCodeMaximum) {
		return core.ErrRunProtocolContract
	}
	if r.Benchmarks != nil {
		if err := r.Benchmarks.Validate(); err != nil {
			return err
		}
	}
	if r.Fuzz != nil {
		if err := r.Fuzz.Validate(); err != nil {
			return err
		}
	}
	if r.Memory != nil {
		if err := r.Memory.Validate(); err != nil {
			return err
		}
	}
	if r.Failure != nil {
		return r.Failure.Validate()
	}
	return nil
}

type RunRetry struct {
	Run               Identifier `json:"run"`
	PassedInIsolation bool       `json:"passed_in_isolation"`
}

func (r RunRetry) Validate() error { return r.Run.Validate() }

// RunArgument is one exact native argv scalar, admitted by process.Argument.
type RunArgument struct{ value process.Argument }

func NewRunArgument(value string) (RunArgument, error) {
	if err := (jsonio.Token{Kind: jsonio.TokenString, Text: value}).Validate(); err != nil {
		return RunArgument{}, contractError(err)
	}
	v, err := process.NewArgument(value)
	if err != nil {
		return RunArgument{}, contractError(err)
	}
	return RunArgument{value: v}, nil
}
func (r RunArgument) Validate() error {
	value, err := r.value.Value()
	if err != nil {
		return contractError(err)
	}
	return contractJoin((jsonio.Token{Kind: jsonio.TokenString, Text: value}).Validate())
}
func (r RunArgument) String() string { v, _ := r.value.Value(); return v }
func (r RunArgument) MarshalJSON() ([]byte, error) {
	v, err := r.value.Value()
	if err != nil {
		return nil, contractError(err)
	}
	return jsonio.MarshalText(v)
}
func (r *RunArgument) UnmarshalJSON(data []byte) error {
	if r == nil {
		return core.ErrRunProtocolContract
	}
	return decodeScalar(data, func(value string) error {
		next, err := NewRunArgument(value)
		if err == nil {
			*r = next
		}
		return err
	})
}

type RunShuffleSeed struct {
	value int64
	set   bool
}

func ParseRunShuffleSeed(text string) (RunShuffleSeed, error) {
	n, err := strconv.ParseInt(text, 10, 64)
	if err != nil || strconv.FormatInt(n, 10) != text {
		return RunShuffleSeed{}, contractError(err)
	}
	return RunShuffleSeed{value: n, set: true}, nil
}
func (r RunShuffleSeed) Validate() error {
	if !r.set {
		return core.ErrRunProtocolContract
	}
	return nil
}
func (r RunShuffleSeed) String() string {
	if !r.set {
		return ""
	}
	return strconv.FormatInt(r.value, 10)
}
func (r RunShuffleSeed) MarshalJSON() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, err
	}
	return jsonio.MarshalText(r.String())
}
func (r *RunShuffleSeed) UnmarshalJSON(data []byte) error {
	if r == nil {
		return core.ErrRunProtocolContract
	}
	return decodeScalar(data, func(value string) error {
		next, err := ParseRunShuffleSeed(value)
		if err == nil {
			*r = next
		}
		return err
	})
}

type RunInvocation struct {
	Tool             core.AbsolutePath `json:"tool"`
	Arguments        []RunArgument     `json:"arguments"`
	Toolchain        Text              `json:"toolchain"`
	Source           core.SHA256Digest `json:"source"`
	JSONV2           bool              `json:"json_v2"`
	Packages         uint16            `json:"packages"`
	Threads          uint16            `json:"threads"`
	MemoryPerProcess core.ByteLength   `json:"memory_per_process"`
}

func (r RunInvocation) Validate() error {
	if len(r.Arguments) == 0 || r.Packages == 0 || r.Threads == 0 || r.MemoryPerProcess.Uint64() == 0 {
		return core.ErrRunProtocolContract
	}
	if err := contractJoin(r.Tool.Validate(), r.Toolchain.Validate(), r.Source.Validate(), r.MemoryPerProcess.Validate()); err != nil {
		return err
	}
	for _, arg := range r.Arguments {
		if err := arg.Validate(); err != nil {
			return err
		}
	}
	return nil
}

type RunFactSequence struct {
	Count  uint64            `json:"count"`
	Bytes  core.ByteLength   `json:"bytes"`
	Digest core.SHA256Digest `json:"digest"`
}

func (r RunFactSequence) Validate() error {
	if err := contractJoin(r.Bytes.Validate(), r.Digest.Validate()); err != nil {
		return err
	}
	if r.Count == 0 && (r.Bytes.Uint64() != 0 || r.Digest != core.SHA256Of(nil)) || r.Count > 0 && (r.Bytes.Uint64() == 0 || r.Count > r.Bytes.Uint64()) {
		return conflictError(nil)
	}
	return nil
}

type RunFact struct {
	Ordinal    uint64                     `json:"ordinal"`
	Kind       RunFactKind                `json:"kind"`
	Invocation *RunInvocation             `json:"invocation,omitempty"`
	Execution  *process.ResultObservation `json:"execution,omitempty"`
	Result     *RunResult                 `json:"result,omitempty"`
}

func (r RunFact) Validate() error {
	if r.Ordinal == 0 {
		return core.ErrRunProtocolContract
	}
	if err := r.Kind.Validate(); err != nil {
		return err
	}
	count := 0
	for _, v := range [...]bool{r.Invocation != nil, r.Execution != nil, r.Result != nil} {
		if v {
			count++
		}
	}
	if count != 1 {
		return conflictError(nil)
	}
	switch r.Kind {
	case RunFactInvocation:
		if r.Invocation != nil {
			return r.Invocation.Validate()
		}
	case RunFactExecution:
		if r.Execution != nil {
			return contractJoin(r.Execution.Validate())
		}
	case RunFactResult:
		if r.Result != nil {
			return r.Result.Validate()
		}
	default:
		return core.ErrRunProtocolContract
	}
	return conflictError(nil)
}

func runKindLabels() []string { return []string{"", "test", "benchmark", "fuzz"} }
func (v RunKind) Validate() error {
	return validateEnum(uint8(v), runKindLabels(), runReceiptKindInvalidDiagnostic)
}
func (v RunKind) IsValid() bool  { return v.Validate() == nil }
func (v RunKind) String() string { return enumString(uint8(v), runKindLabels()) }
func (v RunKind) MarshalJSON() ([]byte, error) {
	return marshalEnum(uint8(v), runKindLabels(), runReceiptKindInvalidDiagnostic)
}
func (v *RunKind) UnmarshalJSON(data []byte) error {
	if v == nil {
		return core.ErrRunProtocolContract
	}
	n, err := unmarshalEnum(data, runKindLabels(), runReceiptKindInvalidDiagnostic)
	if err == nil {
		*v = RunKind(n)
	}
	return err
}
func runSourceKindLabels() []string { return []string{"", "commit", "snapshot"} }
func (v RunSourceKind) Validate() error {
	return validateEnum(uint8(v), runSourceKindLabels(), runReceiptKindInvalidDiagnostic)
}
func (v RunSourceKind) IsValid() bool  { return v.Validate() == nil }
func (v RunSourceKind) String() string { return enumString(uint8(v), runSourceKindLabels()) }
func (v RunSourceKind) MarshalJSON() ([]byte, error) {
	return marshalEnum(uint8(v), runSourceKindLabels(), runReceiptKindInvalidDiagnostic)
}
func (v *RunSourceKind) UnmarshalJSON(data []byte) error {
	if v == nil {
		return core.ErrRunProtocolContract
	}
	n, err := unmarshalEnum(data, runSourceKindLabels(), runReceiptKindInvalidDiagnostic)
	if err == nil {
		*v = RunSourceKind(n)
	}
	return err
}
func runScopeKindLabels() []string { return []string{"", "file", "package", "selected", "project"} }
func (v RunScopeKind) Validate() error {
	return validateEnum(uint8(v), runScopeKindLabels(), runReceiptKindInvalidDiagnostic)
}
func (v RunScopeKind) IsValid() bool  { return v.Validate() == nil }
func (v RunScopeKind) String() string { return enumString(uint8(v), runScopeKindLabels()) }
func (v RunScopeKind) MarshalJSON() ([]byte, error) {
	return marshalEnum(uint8(v), runScopeKindLabels(), runReceiptKindInvalidDiagnostic)
}
func (v *RunScopeKind) UnmarshalJSON(data []byte) error {
	if v == nil {
		return core.ErrRunProtocolContract
	}
	n, err := unmarshalEnum(data, runScopeKindLabels(), runReceiptKindInvalidDiagnostic)
	if err == nil {
		*v = RunScopeKind(n)
	}
	return err
}
func runFactKindLabels() []string { return []string{"", "invocation", "execution", "result"} }
func (v RunFactKind) Validate() error {
	return validateEnum(uint8(v), runFactKindLabels(), runReceiptKindInvalidDiagnostic)
}
func (v RunFactKind) IsValid() bool  { return v.Validate() == nil }
func (v RunFactKind) String() string { return enumString(uint8(v), runFactKindLabels()) }
func (v RunFactKind) MarshalJSON() ([]byte, error) {
	return marshalEnum(uint8(v), runFactKindLabels(), runReceiptKindInvalidDiagnostic)
}
func (v *RunFactKind) UnmarshalJSON(data []byte) error {
	if v == nil {
		return core.ErrRunProtocolContract
	}
	n, err := unmarshalEnum(data, runFactKindLabels(), runReceiptKindInvalidDiagnostic)
	if err == nil {
		*v = RunFactKind(n)
	}
	return err
}
func runResultKindLabels() []string { return []string{"", "package", "test", "benchmark", "fuzz"} }
func (v RunResultKind) Validate() error {
	return validateEnum(uint8(v), runResultKindLabels(), runReceiptKindInvalidDiagnostic)
}
func (v RunResultKind) IsValid() bool  { return v.Validate() == nil }
func (v RunResultKind) String() string { return enumString(uint8(v), runResultKindLabels()) }
func (v RunResultKind) MarshalJSON() ([]byte, error) {
	return marshalEnum(uint8(v), runResultKindLabels(), runReceiptKindInvalidDiagnostic)
}
func (v *RunResultKind) UnmarshalJSON(data []byte) error {
	if v == nil {
		return core.ErrRunProtocolContract
	}
	n, err := unmarshalEnum(data, runResultKindLabels(), runReceiptKindInvalidDiagnostic)
	if err == nil {
		*v = RunResultKind(n)
	}
	return err
}
