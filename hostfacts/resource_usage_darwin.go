//go:build darwin

package hostfacts

import (
	"bytes"
	"context"
	"errors"
	"math"
	"strconv"
	"strings"

	"github.com/deliri/primitive/v2026/core"
)

func observeResourceUsage(ctx context.Context, r ResourceUsageRequest) (ResourceUsage, error) {
	var out ResourceUsage
	var err error
	// macOS has no cgroup CPU counters; scope samples are Linux-only.
	if len(r.Scopes) != 0 {
		return out, core.ErrHostFactsUnsupported
	}
	out.cpus, err = ObserveLogicalCPUCount()
	if err != nil {
		return out, err
	}
	physical, err := ObservePhysicalMemory(ctx)
	if err != nil {
		return out, err
	}
	out.total = physical.TotalBytes()
	top, err := resourceCommand(ctx, "top", "-l", "2", "-s", strconv.FormatUint(uint64(r.IntervalSeconds), 10), "-n", "0")
	if err != nil {
		return out, err
	}
	out.busy, err = parseResourceMacCPU(ctx, top, out.cpus)
	if err != nil {
		return out, err
	}
	vm, err := resourceCommand(ctx, "vm_stat")
	if err != nil {
		return out, err
	}
	out.available, err = parseResourceMacMemory(ctx, vm, out.total)
	if err != nil {
		return out, err
	}
	out.process, err = parseResourceMacProcess(ctx, vm, out.total)
	return out, err
}

func parseResourceMacCPU(ctx context.Context, data []byte, cpus LogicalCPUCount) (uint64, error) {
	found := false
	var busy uint64
	err := resourceLines(ctx, bytes.NewReader(data), func(line []byte) error {
		if !bytes.HasPrefix(line, []byte("CPU usage:")) {
			return nil
		}
		fields := strings.Fields(string(line))
		if len(fields) != 8 || fields[7] != "idle" || !strings.HasSuffix(fields[6], "%") {
			return core.ErrHostFactsObservation
		}
		idle, err := strconv.ParseFloat(strings.TrimSuffix(fields[6], "%"), 64)
		if err != nil || math.IsNaN(idle) || math.IsInf(idle, 0) || idle < 0 || idle > 100 {
			return errors.Join(core.ErrHostFactsObservation, err)
		}
		busy = uint64((100 - idle) * float64(cpus.Int()) * 10)
		found = true
		return nil
	})
	if err != nil || !found || cpus.Validate() != nil {
		return 0, errors.Join(core.ErrHostFactsObservation, err)
	}
	return busy, nil
}

func parseResourceMacMemory(ctx context.Context, data []byte, total core.ByteLength) (core.ByteLength, error) {
	var page uint64
	var pageSeen bool
	var counts [3]uint64
	var seen [3]bool
	err := resourceLines(ctx, bytes.NewReader(data), func(line []byte) error {
		text := string(line)
		if strings.HasPrefix(text, "Mach Virtual Memory Statistics:") {
			if pageSeen {
				return core.ErrHostFactsObservation
			}
			value, ok := strings.CutPrefix(text, "Mach Virtual Memory Statistics: (page size of ")
			if !ok {
				return core.ErrHostFactsObservation
			}
			value, ok = strings.CutSuffix(value, " bytes)")
			if !ok {
				return core.ErrHostFactsObservation
			}
			var err error
			page, err = strconv.ParseUint(value, 10, 64)
			if err != nil || page == 0 {
				return errors.Join(core.ErrHostFactsObservation, err)
			}
			pageSeen = true
			return nil
		}
		key, value, ok := strings.Cut(text, ":")
		if !ok {
			return nil
		}
		index := -1
		switch key {
		case "Pages free":
			index = 0
		case "Pages inactive":
			index = 1
		case "Pages speculative":
			index = 2
		default:
			return nil
		}
		if seen[index] {
			return core.ErrHostFactsObservation
		}
		count, err := strconv.ParseUint(strings.TrimSuffix(strings.TrimSpace(value), "."), 10, 64)
		if err != nil {
			return errors.Join(core.ErrHostFactsObservation, err)
		}
		counts[index] = count
		seen[index] = true
		return nil
	})
	if err != nil || !pageSeen || !seen[0] || !seen[1] || !seen[2] || counts[0] > math.MaxUint64-counts[1] || counts[0]+counts[1] > math.MaxUint64-counts[2] {
		return core.ByteLength{}, errors.Join(core.ErrHostFactsObservation, err)
	}
	pages := counts[0] + counts[1] + counts[2]
	if pages > math.MaxUint64/page || pages*page > total.Uint64() {
		return core.ByteLength{}, core.ErrHostFactsObservation
	}
	return core.NewByteLength(pages * page)
}

// parseResourceMacProcess reads vm_stat's anonymous pages: memory held by
// processes rather than file cache. It needs exactly one page size and one
// anonymous count, and never more than physical memory.
func parseResourceMacProcess(ctx context.Context, data []byte, total core.ByteLength) (core.ByteLength, error) {
	var page, count uint64
	var pageSeen, countSeen bool
	err := resourceLines(ctx, bytes.NewReader(data), func(line []byte) error {
		text := string(line)
		if value, ok := strings.CutPrefix(text, "Mach Virtual Memory Statistics: (page size of "); ok {
			value, ok = strings.CutSuffix(value, " bytes)")
			n, err := strconv.ParseUint(value, 10, 64)
			if !ok || err != nil || n == 0 || pageSeen {
				return errors.Join(core.ErrHostFactsObservation, err)
			}
			page, pageSeen = n, true
			return nil
		}
		key, value, ok := strings.Cut(text, ":")
		if !ok || key != "Anonymous pages" {
			return nil
		}
		n, err := strconv.ParseUint(strings.TrimSuffix(strings.TrimSpace(value), "."), 10, 64)
		if err != nil || countSeen {
			return errors.Join(core.ErrHostFactsObservation, err)
		}
		count, countSeen = n, true
		return nil
	})
	if err != nil || !pageSeen || !countSeen || count > total.Uint64()/page {
		return core.ByteLength{}, errors.Join(core.ErrHostFactsObservation, err)
	}
	return core.NewByteLength(count * page)
}
