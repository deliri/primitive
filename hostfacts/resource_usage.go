package hostfacts

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/process"
	"github.com/deliri/primitive/v2026/temporal"
)

const resourceRecordBytes = 65536
const resourceCPUMilli = 1000
const resourcePercentMilli = 100000
const resourceCommandWaitSeconds = 3

// ResourceUsageRequest selects the interval between native CPU observations.
// Scopes names Linux systemd scopes whose cgroup CPU is sampled strictly
// inside the same host window, so a caller can separate its own load from
// background load. It sets no threshold, reservation, limit or admission policy.
type ResourceUsageRequest struct {
	IntervalSeconds uint32
	Scopes          []SystemdScopeRequest
}

func (r ResourceUsageRequest) Validate() error {
	if r.IntervalSeconds == 0 {
		return core.ErrHostFactsContract
	}
	if _, err := temporal.DurationFromSeconds(uint64(r.IntervalSeconds)); err != nil {
		return err
	}
	for i, scope := range r.Scopes {
		if err := scope.Validate(); err != nil {
			return err
		}
		// A repeated scope would be counted twice by a caller's subtraction.
		if slices.Contains(r.Scopes[:i], scope) {
			return core.ErrHostFactsContract
		}
	}
	return nil
}

// ScopeCPU is one scope's cumulative cgroup CPU delta, taken strictly inside
// the host window and divided by the whole host window. It therefore never
// exceeds the scope's CPU during the host sample. Present is false when the
// scope or its cgroup was absent at either sample; BusyCPUMilli is then zero.
type ScopeCPU struct {
	Scope        SystemdScopeRequest
	BusyCPUMilli uint64
	Present      bool
}

// ResourceUsage is one immutable interval CPU sample and current memory/pressure
// observation. It does not reserve resources or supervise a workload.
type ResourceUsage struct {
	cpus                       LogicalCPUCount
	busy                       uint64
	total, available           core.ByteLength
	ioPressure, memoryPressure uint32
	scopes                     []ScopeCPU
}

func (u ResourceUsage) Validate() error {
	if err := errors.Join(u.cpus.Validate(), u.total.Validate(), u.available.Validate()); err != nil {
		return errors.Join(core.ErrHostFactsContract, err)
	}
	limit := uint64(u.cpus.Int()) * resourceCPUMilli
	if u.total.Uint64() == 0 || u.available.Uint64() > u.total.Uint64() || u.busy > limit || u.ioPressure > resourcePercentMilli || u.memoryPressure > resourcePercentMilli {
		return core.ErrHostFactsContract
	}
	for _, scope := range u.scopes {
		if err := scope.Scope.Validate(); err != nil {
			return errors.Join(core.ErrHostFactsContract, err)
		}
		if scope.BusyCPUMilli > limit || !scope.Present && scope.BusyCPUMilli != 0 {
			return core.ErrHostFactsContract
		}
	}
	return nil
}

// Scopes returns the requested scope samples in request order.
func (u ResourceUsage) Scopes() []ScopeCPU                    { return slices.Clone(u.scopes) }
func (u ResourceUsage) CPUCount() LogicalCPUCount             { return u.cpus }
func (u ResourceUsage) BusyCPUMilli() uint64                  { return u.busy }
func (u ResourceUsage) TotalMemoryBytes() core.ByteLength     { return u.total }
func (u ResourceUsage) AvailableMemoryBytes() core.ByteLength { return u.available }
func (u ResourceUsage) IOPressureMilli() uint32               { return u.ioPressure }
func (u ResourceUsage) MemoryPressureMilli() uint32           { return u.memoryPressure }

// ObserveResourceUsage samples the current host. Cancellation and observation
// failure return no sample. Other workloads are naturally included in OS facts.
func ObserveResourceUsage(ctx context.Context, r ResourceUsageRequest) (ResourceUsage, error) {
	if err := errors.Join(contextstate.Validate(ctx), r.Validate()); err != nil {
		return ResourceUsage{}, fail(OperationResourceUsage, core.ErrHostFactsContract, err)
	}
	u, err := observeResourceUsage(ctx, r)
	if err == nil {
		err = errors.Join(u.Validate(), contextstate.Validate(ctx), sameScopes(r.Scopes, u.scopes))
	}
	if err != nil {
		return ResourceUsage{}, fail(OperationResourceUsage, core.ErrHostFactsObservation, err)
	}
	return u, nil
}

// Every requested scope is answered exactly once, in request order.
func sameScopes(requested []SystemdScopeRequest, observed []ScopeCPU) error {
	if len(requested) != len(observed) {
		return core.ErrHostFactsObservation
	}
	for i := range requested {
		if observed[i].Scope != requested[i] {
			return core.ErrHostFactsObservation
		}
	}
	return nil
}

// Each recognized native record has finitely many bounded numeric fields.
// Unknown records are drained across any number of fixed windows. The total
// source extent is unrestricted; native read errors remain visible.
func resourceLines(ctx context.Context, source io.Reader, visit func([]byte) error) error {
	r := bufio.NewReaderSize(source, resourceRecordBytes)
	for {
		if err := contextstate.Validate(ctx); err != nil {
			return err
		}
		line, readErr := r.ReadSlice('\n')
		if readErr == bufio.ErrBufferFull {
			known := bytes.HasPrefix(line, []byte("cpu ")) || bytes.HasPrefix(line, []byte("MemTotal:")) || bytes.HasPrefix(line, []byte("MemAvailable:")) || bytes.HasPrefix(line, []byte("some "))
			if known {
				return core.ErrHostFactsObservation
			}
			for readErr == bufio.ErrBufferFull {
				if err := contextstate.Validate(ctx); err != nil {
					return err
				}
				_, readErr = r.ReadSlice('\n')
			}
		} else if len(line) > 0 {
			if err := visit(bytes.TrimSuffix(line, []byte{'\n'})); err != nil {
				return err
			}
		}
		if readErr == io.EOF {
			return nil
		}
		if readErr != nil {
			return readErr
		}
	}
}

func resourceFile(ctx context.Context, path string, visit func([]byte) error) (resultErr error) {
	abs, err := core.ParseAbsolutePath(path)
	if err != nil {
		return err
	}
	location, err := filestore.OpenParent(ctx, abs)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, location.Root.Close()) }()
	file, err := filestore.OpenRead(ctx, filestore.ReadHandleRequest{Location: location})
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, file.Close()) }()
	return resourceLines(ctx, file, visit)
}

func resourceCommand(ctx context.Context, name string, args ...string) ([]byte, error) {
	commandName, err := core.ParsePathComponent(name)
	if err != nil {
		return nil, err
	}
	command, err := process.Resolve(ctx, commandName)
	if err != nil {
		return nil, err
	}
	dir, err := WorkingDirectory()
	if err != nil {
		return nil, err
	}
	argv, err := process.ParseArguments(args)
	if err != nil {
		return nil, err
	}
	env, err := AmbientEnvironment()
	if err != nil {
		return nil, err
	}
	localeName, err := process.NewEnvironmentName("LC_ALL")
	if err != nil {
		return nil, err
	}
	localeValue, err := process.NewEnvironmentValue("C")
	if err != nil {
		return nil, err
	}
	env, err = env.With(process.EnvironmentVariable{Name: localeName, Value: localeValue})
	if err != nil {
		return nil, err
	}
	maximum, err := core.NewByteCount(resourceRecordBytes)
	if err != nil {
		return nil, err
	}
	var stdout, stderr bytes.Buffer
	// iostat -n 0 and vm_stat emit fixed host-summary schemas, not process lists.
	// Their finite numeric metadata is bounded; no test/evidence stream enters.
	waitDelay, err := temporal.DurationFromSeconds(resourceCommandWaitSeconds)
	if err != nil {
		return nil, err
	}
	result, err := process.Run(ctx, process.Request{Command: command, Arguments: argv, WorkingDirectory: dir, Environment: env, Streams: process.Streams{Stdin: strings.NewReader(""), Stdout: &stdout, Stderr: &stderr}, OutputPolicy: process.OutputPolicy{Mode: process.OutputModeBounded, Maximum: maximum}, WaitDelay: waitDelay, Containment: process.Containment{Isolation: process.IsolationDirect, CancelSignal: process.CancelSignalKill}})
	if err != nil {
		return nil, err
	}
	exit, err := result.ExitCode()
	if err != nil {
		return nil, err
	}
	code, err := exit.Int()
	if err != nil {
		return nil, err
	}
	if code != 0 {
		return nil, core.ErrHostFactsObservation
	}
	return stdout.Bytes(), nil
}

type resourceCPUCounters struct{ total, idle uint64 }

func parseResourceCPU(line []byte) (resourceCPUCounters, error) {
	fields := strings.Fields(string(line))
	if len(fields) < 9 || fields[0] != "cpu" {
		return resourceCPUCounters{}, core.ErrHostFactsObservation
	}
	var out resourceCPUCounters
	for i, value := range fields[1:9] {
		n, err := strconv.ParseUint(value, 10, 64)
		if err != nil || math.MaxUint64-out.total < n {
			return resourceCPUCounters{}, errors.Join(core.ErrHostFactsObservation, err)
		}
		out.total += n
		if i == 3 || i == 4 {
			out.idle += n
		}
	}
	if out.total == 0 || out.idle > out.total {
		return resourceCPUCounters{}, core.ErrHostFactsObservation
	}
	return out, nil
}

func resourceBusy(before, after resourceCPUCounters, cpus LogicalCPUCount) (uint64, error) {
	if cpus.Validate() != nil || after.total <= before.total || after.idle < before.idle || after.idle-before.idle > after.total-before.total {
		return 0, core.ErrHostFactsObservation
	}
	total := after.total - before.total
	busy := total - (after.idle - before.idle)
	return uint64(float64(busy) / float64(total) * float64(cpus.Int()) * resourceCPUMilli), nil
}

func parseResourcePressure(ctx context.Context, source io.Reader) (uint32, error) {
	found := false
	var value uint32
	err := resourceLines(ctx, source, func(line []byte) error {
		if !bytes.HasPrefix(line, []byte("some ")) {
			return nil
		}
		if found {
			return core.ErrHostFactsObservation
		}
		for _, field := range strings.Fields(string(line)) {
			text, ok := strings.CutPrefix(field, "avg10=")
			if !ok {
				continue
			}
			if found {
				return core.ErrHostFactsObservation
			}
			n, err := strconv.ParseFloat(text, 64)
			if err != nil || math.IsNaN(n) || math.IsInf(n, 0) || n < 0 || n > 100 {
				return errors.Join(core.ErrHostFactsObservation, err)
			}
			value = uint32(math.Round(n * 1000))
			found = true
		}
		return nil
	})
	if err != nil || !found {
		return 0, errors.Join(core.ErrHostFactsObservation, err)
	}
	return value, nil
}
