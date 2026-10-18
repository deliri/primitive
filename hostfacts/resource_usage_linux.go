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
	before, err := readResourceCPU(ctx)
	if err != nil {
		return out, err
	}
	duration, err := temporal.DurationFromSeconds(uint64(r.IntervalSeconds))
	if err != nil {
		return out, err
	}
	if err = temporal.Wait(temporal.WaitRequest{Context: ctx, Duration: duration}); err != nil {
		return out, err
	}
	after, err := readResourceCPU(ctx)
	if err != nil {
		return out, err
	}
	out.busy, err = resourceBusy(before, after, out.cpus)
	if err != nil {
		return out, err
	}
	out.total, out.available, err = readResourceMemory(ctx)
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

func readResourceMemory(ctx context.Context) (core.ByteLength, core.ByteLength, error) {
	var total, available uint64
	var totalSeen, availableSeen bool
	err := resourceFile(ctx, "/proc/meminfo", func(line []byte) error {
		fields := strings.Fields(string(line))
		if len(fields) == 0 || fields[0] != "MemTotal:" && fields[0] != "MemAvailable:" {
			return nil
		}
		if len(fields) != 3 || fields[2] != "kB" {
			return core.ErrHostFactsObservation
		}
		n, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil || n > math.MaxUint64/1024 {
			return errors.Join(core.ErrHostFactsObservation, err)
		}
		if fields[0] == "MemTotal:" {
			if totalSeen {
				return core.ErrHostFactsObservation
			}
			totalSeen = true
			total = n * 1024
		} else {
			if availableSeen {
				return core.ErrHostFactsObservation
			}
			availableSeen = true
			available = n * 1024
		}
		return nil
	})
	if err != nil || !totalSeen || !availableSeen || total == 0 || available > total {
		return core.ByteLength{}, core.ByteLength{}, errors.Join(core.ErrHostFactsObservation, err)
	}
	totalBytes, err := core.NewByteLength(total)
	if err != nil {
		return core.ByteLength{}, core.ByteLength{}, err
	}
	availableBytes, err := core.NewByteLength(available)
	return totalBytes, availableBytes, err
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
