//go:build linux

package hostfacts

import (
	"bytes"
	"context"
	"errors"
	"math"
	"strconv"
	"strings"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

func observeResourceUsage(ctx context.Context, r ResourceUsageRequest) (ResourceUsage, error) {
	var out ResourceUsage
	var err error
	out.cpus, err = ObserveLogicalCPUCount()
	if err != nil {
		return out, err
	}
	mounts, err := procSelfMountsPath()
	if err != nil {
		return out, err
	}
	cgroups, err := resolveScopeCgroups(ctx, mounts, r.Scopes)
	if err != nil {
		return out, err
	}
	duration, err := temporal.DurationFromSeconds(uint64(r.IntervalSeconds))
	if err != nil {
		return out, err
	}
	// Scope samples sit strictly inside the two host samples, and the clock
	// brackets both host samples, so no scope can claim more than its window.
	start, err := temporal.Observe()
	if err != nil {
		return out, err
	}
	before, err := readResourceCPU(ctx)
	if err != nil {
		return out, err
	}
	scopeBefore, err := readScopesCPU(ctx, cgroups)
	if err != nil {
		return out, err
	}
	if err = temporal.Wait(temporal.WaitRequest{Context: ctx, Duration: duration}); err != nil {
		return out, err
	}
	scopeAfter, err := readScopesCPU(ctx, cgroups)
	if err != nil {
		return out, err
	}
	after, err := readResourceCPU(ctx)
	if err != nil {
		return out, err
	}
	end, err := temporal.Observe()
	if err != nil {
		return out, err
	}
	window, err := end.Since(start)
	if err != nil {
		return out, err
	}
	out.busy, err = resourceBusy(before, after, out.cpus)
	if err != nil {
		return out, err
	}
	out.scopes, err = scopesBusy(cgroups, scopeBefore, scopeAfter, window.Nanoseconds())
	if err != nil {
		return out, err
	}
	out.total, out.available, out.process, err = readResourceMemory(ctx)
	if err != nil {
		return out, err
	}
	out.ioPressure, err = readResourcePressure(ctx, "/proc/pressure/io")
	if err != nil {
		return out, err
	}
	out.memoryPressure, err = readResourcePressure(ctx, "/proc/pressure/memory")
	return out, err
}

func readResourceCPU(ctx context.Context) (resourceCPUCounters, error) {
	var counters resourceCPUCounters
	found := false
	err := resourceFile(ctx, "/proc/stat", func(line []byte) error {
		if !bytes.HasPrefix(line, []byte("cpu ")) {
			return nil
		}
		if found {
			return core.ErrHostFactsObservation
		}
		found = true
		var err error
		counters, err = parseResourceCPU(line)
		return err
	})
	if err != nil || !found {
		return resourceCPUCounters{}, errors.Join(core.ErrHostFactsObservation, err)
	}
	return counters, nil
}

func readResourceMemory(ctx context.Context) (core.ByteLength, core.ByteLength, core.ByteLength, error) {
	// MemTotal, MemAvailable and AnonPages, each exactly once.
	var values [3]uint64
	var seen [3]bool
	err := resourceFile(ctx, "/proc/meminfo", func(line []byte) error {
		fields := strings.Fields(string(line))
		index := -1
		if len(fields) > 0 {
			switch fields[0] {
			case "MemTotal:":
				index = 0
			case "MemAvailable:":
				index = 1
			case "AnonPages:":
				index = 2
			}
		}
		if index < 0 {
			return nil
		}
		if len(fields) != 3 || fields[2] != "kB" || seen[index] {
			return core.ErrHostFactsObservation
		}
		n, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil || n > math.MaxUint64/1024 {
			return errors.Join(core.ErrHostFactsObservation, err)
		}
		values[index], seen[index] = n*1024, true
		return nil
	})
	total, available, process := values[0], values[1], values[2]
	if err != nil || !seen[0] || !seen[1] || !seen[2] || total == 0 || available > total || process > total {
		return core.ByteLength{}, core.ByteLength{}, core.ByteLength{}, errors.Join(core.ErrHostFactsObservation, err)
	}
	totalBytes, err := core.NewByteLength(total)
	if err != nil {
		return core.ByteLength{}, core.ByteLength{}, core.ByteLength{}, err
	}
	availableBytes, err := core.NewByteLength(available)
	if err != nil {
		return core.ByteLength{}, core.ByteLength{}, core.ByteLength{}, err
	}
	processBytes, err := core.NewByteLength(process)
	return totalBytes, availableBytes, processBytes, err
}

func readResourcePressure(ctx context.Context, path string) (uint32, error) {
	var pressure uint32
	found := false
	err := resourceFile(ctx, path, func(line []byte) error {
		if !bytes.HasPrefix(line, []byte("some ")) {
			return nil
		}
		if found {
			return core.ErrHostFactsObservation
		}
		found = true
		var err error
		pressure, err = parseResourcePressure(ctx, bytes.NewReader(line))
		return err
	})
	if err != nil || !found {
		return 0, errors.Join(core.ErrHostFactsObservation, err)
	}
	return pressure, nil
}
