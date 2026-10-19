package hostfacts

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"math/bits"
	"strconv"
	"strings"

	"github.com/deliri/primitive/v2026/core"
)

// The kernel charges CPU time to a cgroup's own counters as its tasks run, so
// cpu.stat usage_usec is cumulative and retains time from exited processes.
const (
	cgroupCPUStatName        = "cpu.stat"
	cgroupCPUUsageKey        = "usage_usec"
	cgroupCPUStatBytes       = 4 << 10
	scopeMicrosecondsToMilli = 1_000_000
)

// scopeCgroup is one requested scope's resolved cpu.stat file. present is false
// when the manager reports no control group for the unit.
type scopeCgroup struct {
	scope   SystemdScopeRequest
	stat    core.AbsolutePath
	present bool
}

// resolveScopeCgroups asks systemd for each unit's ControlGroup and places it
// under the cgroup v2 mount that contains it.
func resolveScopeCgroups(ctx context.Context, mountsPath core.AbsolutePath, scopes []SystemdScopeRequest) ([]scopeCgroup, error) {
	resolved := make([]scopeCgroup, 0, len(scopes))
	for _, scope := range scopes {
		args := []string{"show", "--property=ControlGroup", "--value", "--", scope.Unit.String()}
		if scope.UserManager {
			args = append([]string{"--user"}, args...)
		}
		data, err := resourceCommand(ctx, "systemctl", args...)
		if err != nil {
			return nil, err
		}
		group, err := parseSystemdControlGroup(data)
		if err != nil {
			return nil, err
		}
		if group == "" {
			resolved = append(resolved, scopeCgroup{scope: scope})
			continue
		}
		membership := cgroupMembership{path: group, source: WorkloadMemoryLimitSourceCgroupV2, scope: cgroupMembershipInside}
		mount, err := selectCgroupMount(ctx, mountsPath, membership)
		if err != nil {
			return nil, err
		}
		directory, err := resolveCgroupDirectory(group, mount)
		if err != nil {
			return nil, err
		}
		name, err := core.ParsePathComponent(cgroupCPUStatName)
		if err != nil {
			return nil, err
		}
		stat, err := directory.Join(name)
		if err != nil {
			return nil, err
		}
		resolved = append(resolved, scopeCgroup{scope: scope, stat: stat, present: true})
	}
	return resolved, nil
}

// parseSystemdControlGroup admits systemctl's single-value output. An empty
// value means the manager has no control group for the unit.
func parseSystemdControlGroup(data []byte) (string, error) {
	value, _ := strings.CutSuffix(string(data), "\n")
	if value == "" {
		return "", nil
	}
	if !validCgroupPath(value) || strings.ContainsAny(value, "\n\r") {
		return "", core.ErrHostFactsObservation
	}
	return value, nil
}

// readScopeCPU returns cumulative usage_usec. A cgroup that disappeared is
// absent rather than an error; its usage cannot be attributed to the window.
func readScopeCPU(ctx context.Context, cgroup scopeCgroup) (uint64, bool, error) {
	if !cgroup.present {
		return 0, false, nil
	}
	var fold cpuStatFold
	err := scanVirtualLines(ctx, virtualFileRequest{Path: cgroup.stat, MaximumBytes: cgroupCPUStatBytes}, fold.visit)
	if errors.Is(err, fs.ErrNotExist) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	if !fold.seen {
		return 0, false, core.ErrHostFactsObservation
	}
	return fold.usage, true, nil
}

// cpuStatFold reads the flat-keyed cpu.stat file and keeps only usage_usec.
type cpuStatFold struct {
	usage uint64
	seen  bool
}

func (f *cpuStatFold) visit(line []byte) error {
	key, value, found := bytes.Cut(line, []byte{' '})
	if !found || len(key) == 0 {
		return core.ErrHostFactsObservation
	}
	if string(key) != cgroupCPUUsageKey {
		return nil
	}
	if f.seen || len(value) == 0 || value[0] < '0' || value[0] > '9' {
		return core.ErrHostFactsObservation
	}
	usage, err := strconv.ParseUint(string(value), 10, 64)
	if err != nil {
		return errors.Join(core.ErrHostFactsObservation, err)
	}
	f.usage, f.seen = usage, true
	return nil
}

// scopeSample is one cumulative cpu.stat reading; present is false for a scope
// without a control group or whose cgroup was gone at the read.
type scopeSample struct {
	usage   uint64
	present bool
}

func readScopesCPU(ctx context.Context, cgroups []scopeCgroup) ([]scopeSample, error) {
	samples := make([]scopeSample, 0, len(cgroups))
	for _, cgroup := range cgroups {
		usage, present, err := readScopeCPU(ctx, cgroup)
		if err != nil {
			return nil, err
		}
		samples = append(samples, scopeSample{usage: usage, present: present})
	}
	return samples, nil
}

// scopesBusy pairs each scope's two readings. A scope absent at either read
// reports zero, which can only overstate the caller's background load.
func scopesBusy(cgroups []scopeCgroup, before, after []scopeSample, windowNanoseconds int64) ([]ScopeCPU, error) {
	if len(before) != len(cgroups) || len(after) != len(cgroups) {
		return nil, core.ErrHostFactsObservation
	}
	scopes := make([]ScopeCPU, 0, len(cgroups))
	for i, cgroup := range cgroups {
		result := ScopeCPU{Scope: cgroup.scope}
		if before[i].present && after[i].present {
			milli, present, err := scopeBusyMilli(before[i].usage, after[i].usage, windowNanoseconds)
			if err != nil {
				return nil, err
			}
			result.BusyCPUMilli, result.Present = milli, present
		}
		scopes = append(scopes, result)
	}
	return scopes, nil
}

// scopeBusyMilli divides a scope's CPU delta by the whole host window and
// rounds down, so the result never exceeds the scope's use of that window.
// A decreasing counter means the cgroup was replaced and is reported absent.
func scopeBusyMilli(before, after uint64, windowNanoseconds int64) (uint64, bool, error) {
	if windowNanoseconds <= 0 {
		return 0, false, core.ErrHostFactsObservation
	}
	if after < before {
		return 0, false, nil
	}
	high, low := bits.Mul64(after-before, scopeMicrosecondsToMilli)
	window := uint64(windowNanoseconds)
	if high >= window {
		return 0, false, core.ErrHostFactsObservation
	}
	milli, _ := bits.Div64(high, low, window)
	return milli, true, nil
}
