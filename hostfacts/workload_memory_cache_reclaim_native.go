package hostfacts

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"strconv"
	"strings"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

// Native cgroup directories are streamed one at a time. No process population
// or reclaim credit survives this call. A populated ancestor is only traversed.
func reclaimEmptyCgroupCache(ctx context.Context, root *os.Root, maximum uint64) (written bool, resultErr error) {
	if err := contextstate.Validate(ctx); err != nil {
		return false, err
	}
	empty, err := emptyCgroupCache(ctx, root, maximum)
	if err != nil {
		return false, err
	}
	if empty {
		return requestEmptyCgroupCacheReclaim(ctx, root, maximum)
	}
	return reclaimCgroupChildren(ctx, root, maximum)
}

func reclaimCgroupChildren(ctx context.Context, root *os.Root, maximum uint64) (written bool, resultErr error) {
	directory, err := root.Open(".")
	if err != nil {
		return false, err
	}
	defer func() { resultErr = errors.Join(resultErr, directory.Close()) }()
	for {
		rows, readErr := directory.ReadDir(1)
		for _, row := range rows {
			wrote, err := reclaimCgroupChild(ctx, root, row, maximum)
			if wrote || err != nil {
				return wrote, err
			}
		}
		if errors.Is(readErr, io.EOF) {
			return false, nil
		}
		if readErr != nil {
			return false, readErr
		}
	}
}

func reclaimCgroupChild(ctx context.Context, root *os.Root, row fs.DirEntry, maximum uint64) (bool, error) {
	if !row.IsDir() || row.Type()&os.ModeSymlink != 0 {
		return false, nil
	}
	component, err := core.ParseRelativePath(row.Name())
	if err != nil {
		return false, err
	}
	child, err := filestore.OpenChildRoot(ctx, filestore.ChildRootRequest{Location: filestore.Location{Root: root, Path: component}})
	if err != nil {
		return false, err
	}
	wrote, reclaimErr := reclaimEmptyCgroupCache(ctx, child, maximum)
	return wrote, errors.Join(reclaimErr, filestore.CloseRoot(filestore.RootHandleRequest{Root: child}))
}

func emptyCgroupCache(ctx context.Context, root *os.Root, maximum uint64) (bool, error) {
	events, err := readCacheControl(ctx, root, "cgroup.events")
	if err != nil {
		return false, err
	}
	populated, err := cgroupCacheField(events, "populated")
	if err != nil {
		return false, err
	}
	if populated > 1 {
		return false, core.ErrHostFactsObservation
	}
	if populated != 0 {
		return false, nil
	}
	stat, err := readCacheControl(ctx, root, "memory.stat")
	if err != nil {
		return false, err
	}
	for _, name := range [...]string{"anon", "shmem", "file_dirty", "file_writeback"} {
		value, err := cgroupCacheField(stat, name)
		if err != nil {
			return false, err
		}
		if value != 0 {
			return false, nil
		}
	}
	inactive, err := cgroupCacheField(stat, "inactive_file")
	return inactive >= maximum, err
}

func requestEmptyCgroupCacheReclaim(ctx context.Context, root *os.Root, maximum uint64) (written bool, resultErr error) {
	file, err := root.OpenFile("memory.reclaim", os.O_WRONLY, 0)
	if err != nil {
		return false, err
	}
	defer func() { resultErr = errors.Join(resultErr, file.Close()) }()
	// Opening the control must not turn an earlier empty observation into authority
	// to reclaim a newly populated or dirty group.
	empty, err := emptyCgroupCache(ctx, root, maximum)
	if err != nil || !empty {
		return false, err
	}
	if err := contextstate.Validate(ctx); err != nil {
		return false, err
	}
	data := strconv.FormatUint(maximum, 10) + " swappiness=0"
	n, err := file.WriteString(data)
	if err == nil && n != len(data) {
		err = io.ErrShortWrite
	}
	return true, err
}

func readCacheControl(ctx context.Context, root *os.Root, name string) ([]byte, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return nil, err
	}
	file, err := root.Open(name)
	if err != nil {
		return nil, err
	}
	data, readErr := readBoundedValue(ctx, file, 32<<10)
	return data, errors.Join(readErr, file.Close())
}

func cgroupCacheField(data []byte, name string) (uint64, error) {
	var value uint64
	found := false
	for line := range strings.SplitSeq(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 || fields[0] != name {
			continue
		}
		if found || len(fields) != 2 {
			return 0, core.ErrHostFactsObservation
		}
		number, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil || strconv.FormatUint(number, 10) != fields[1] {
			return 0, core.ErrHostFactsObservation
		}
		value, found = number, true
	}
	if !found {
		return 0, core.ErrHostFactsObservation
	}
	return value, nil
}
