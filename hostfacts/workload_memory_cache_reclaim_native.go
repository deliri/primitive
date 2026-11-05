package hostfacts

import (
	"context"
	"errors"
	"os"
	"strconv"
	"strings"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

// Native cgroup directories are streamed one at a time. No process population
// or reclaim credit survives this call. A populated ancestor is only traversed.
func reclaimEmptyCgroupCache(ctx context.Context, root *os.Root, maximum uint64) (bool, error) {
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

func reclaimCgroupChildren(ctx context.Context, root *os.Root, maximum uint64) (bool, error) {
	var written bool
	path, err := core.ParseRelativePath(".")
	if err != nil {
		return false, err
	}
	walkErr := filestore.Walk(ctx, filestore.WalkRequest{Location: filestore.Location{Root: root, Path: path}, Visit: func(entry filestore.WalkEntry) (filestore.WalkDirective, error) {
		if written || !entry.Entry.IsDir() || entry.Entry.Type()&os.ModeSymlink != 0 {
			return filestore.WalkSkipDirectory, nil
		}
		child, reclaimErr := filestore.OpenChildRoot(ctx, filestore.ChildRootRequest{Location: filestore.Location{Root: root, Path: entry.Path}})
		if reclaimErr != nil {
			return filestore.WalkSkipDirectory, reclaimErr
		}
		// Walk owns recursive directory iteration; this visitor tests only this root.
		empty, checkErr := emptyCgroupCache(ctx, child, maximum)
		if checkErr == nil && empty {
			written, checkErr = requestEmptyCgroupCacheReclaim(ctx, child, maximum)
		}
		closeErr := filestore.CloseRoot(filestore.RootHandleRequest{Root: child})
		if written {
			return filestore.WalkSkipDirectory, errors.Join(checkErr, closeErr)
		}
		return filestore.WalkContinue, errors.Join(checkErr, closeErr)
	}})
	return written, walkErr
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
		value, fieldErr := cgroupCacheField(stat, name)
		if fieldErr != nil {
			return false, fieldErr
		}
		if value != 0 {
			return false, nil
		}
	}
	inactive, err := cgroupCacheField(stat, "inactive_file")
	return inactive >= maximum, err
}

func requestEmptyCgroupCacheReclaim(ctx context.Context, root *os.Root, maximum uint64) (written bool, resultErr error) {
	path, err := core.ParseRelativePath("memory.reclaim")
	if err != nil {
		return false, err
	}
	file, err := filestore.OpenWriteExisting(ctx, filestore.UpdateHandleRequest{Location: filestore.Location{Root: root, Path: path}})
	if err != nil {
		return false, err
	}
	defer func() {
		resultErr = errors.Join(resultErr, filestore.CloseFile(filestore.FileHandleRequest{File: file}))
	}()
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
	_, err = filestore.WriteChunk(ctx, filestore.ChunkWriteRequest{Destination: file, Bytes: []byte(data)})
	return true, err
}

func readCacheControl(ctx context.Context, root *os.Root, name string) ([]byte, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return nil, err
	}
	path, err := core.ParseRelativePath(name)
	if err != nil {
		return nil, err
	}
	file, err := filestore.OpenRead(ctx, filestore.ReadHandleRequest{Location: filestore.Location{Root: root, Path: path}})
	if err != nil {
		return nil, err
	}
	data, readErr := readBoundedValue(ctx, file, 32<<10)
	return data, errors.Join(readErr, filestore.CloseFile(filestore.FileHandleRequest{File: file}))
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
