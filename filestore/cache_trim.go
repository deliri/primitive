package filestore

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"math/bits"
	"os"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lineio"
	"github.com/deliri/primitive/v2026/temporal"
)

// One record per regular file: hex(mtime with flipped sign bit) SP hex(path)
// SP hex(size) LF. Hex keeps LF-bearing names framable and preserves raw byte
// order, and SP sorts below every hex digit, so bytewise record order is
// oldest first with ties broken by path.
const (
	cacheTrimRecordsName = "cache-records"
	cacheTrimFieldBytes  = 16
	cacheTrimFixedBytes  = 2*cacheTrimFieldBytes + 3
)

// CacheTrimRequest names a cache tree, its byte limit and a scratch parent
// for the eviction order. Parent must lie outside the cache tree.
type CacheTrimRequest struct {
	Location Location
	Parent   core.AbsolutePath
	Limit    core.ByteCount
}

// Validate rejects an unset location or scratch parent and a zero limit.
func (r CacheTrimRequest) Validate() error {
	if err := r.Location.Validate(); err != nil {
		return err
	}
	if err := r.Parent.Validate(); err != nil {
		return contractError(err)
	}
	if err := r.Limit.Validate(); err != nil {
		return contractError(err)
	}
	return nil
}

// CacheTrimObservation reports regular-file bytes observed by the walk, the
// bytes remaining after eviction by that same accounting, and files removed.
type CacheTrimObservation struct {
	BeforeBytes  uint64
	AfterBytes   uint64
	RemovedFiles uint64
}

// TrimCache evicts least-recently-modified regular files under Location until
// their total size is at most Limit. One streaming Walk lstats each entry;
// when the total is within Limit nothing is sorted or removed. Otherwise the
// observed files are externally sorted under Parent by modification time,
// ties by path, and removed oldest first until the observed total fits.
// Working memory does not grow with the number of files.
//
// Directories, symbolic links and special files are neither counted nor
// removed; links are not followed. A file that vanishes after the walk saw it
// counts as evicted without being reported as removed. The guarantee covers
// the files this call observed: files a concurrent writer adds or grows during
// the call are not accounted for until the next call. Removal does not
// synchronize parents; a cache entry needs no durable deletion.
func TrimCache(ctx context.Context, request CacheTrimRequest) (CacheTrimObservation, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return CacheTrimObservation{}, err
	}
	if err := request.Validate(); err != nil {
		return CacheTrimObservation{}, err
	}
	var observation CacheTrimObservation
	err := WithScratchScope(ctx, ScratchScopeRequest{Parent: request.Parent, Use: func(ctx context.Context, scratch *os.Root) (resultErr error) {
		records, err := scratch.OpenFile(cacheTrimRecordsName, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
		if err != nil {
			return activationError(err)
		}
		defer func() {
			if err := records.Close(); err != nil {
				resultErr = errors.Join(resultErr, cleanupError(err))
			}
		}()
		observation, err = trimCacheRecords(ctx, request, records)
		return err
	}})
	if err != nil {
		return CacheTrimObservation{}, err
	}
	return observation, nil
}

func trimCacheRecords(ctx context.Context, request CacheTrimRequest, records *os.File) (CacheTrimObservation, error) {
	var observation CacheTrimObservation
	writer := bufio.NewWriterSize(records, contentSortBufferBytes)
	var record []byte
	err := Walk(ctx, WalkRequest{Location: request.Location, Visit: func(entry WalkEntry) (WalkDirective, error) {
		if !entry.Entry.Type().IsRegular() {
			return WalkContinue, nil
		}
		info, err := entry.Entry.Info()
		if errors.Is(err, fs.ErrNotExist) {
			return WalkContinue, nil
		}
		if err != nil {
			return WalkContinue, sourceError(err)
		}
		if !info.Mode().IsRegular() {
			return WalkContinue, nil
		}
		size, err := core.CheckedUint64FromInt64(info.Size())
		if err != nil {
			return WalkContinue, sourceError(err)
		}
		record, err = appendCacheTrimRecord(record[:0], entry.Path, info, size)
		if err != nil {
			return WalkContinue, err
		}
		total, carry := bits.Add64(observation.BeforeBytes, size, 0)
		if carry != 0 {
			return WalkContinue, sizeError(errors.New("filestore cache size exceeds uint64"))
		}
		observation.BeforeBytes = total
		if _, err := writer.Write(record); err != nil {
			return WalkContinue, destinationError(err)
		}
		return WalkContinue, nil
	}})
	if err != nil {
		return CacheTrimObservation{}, err
	}
	observation.AfterBytes = observation.BeforeBytes
	limit, err := request.Limit.Uint64()
	if err != nil {
		return CacheTrimObservation{}, contractError(err)
	}
	if observation.BeforeBytes <= limit {
		return observation, nil
	}
	if err := writer.Flush(); err != nil {
		return CacheTrimObservation{}, destinationError(err)
	}
	info, err := records.Stat()
	if err != nil {
		return CacheTrimObservation{}, sourceError(err)
	}
	source := io.NewSectionReader(records, 0, info.Size())
	var left, right []byte
	_, err = SortRecordStream(ctx, RecordRangeStreamRequest{
		Parent: request.Parent,
		Source: source,
		Compare: func(_ context.Context, source lineio.RecordSource, a, b lineio.RecordRange) (core.Comparison, error) {
			var err error
			if left, err = readCacheTrimRecord(source, a, left); err != nil {
				return core.ComparisonUnknown, err
			}
			if right, err = readCacheTrimRecord(source, b, right); err != nil {
				return core.ComparisonUnknown, err
			}
			switch bytes.Compare(left, right) {
			case -1:
				return core.ComparisonLess, nil
			case 1:
				return core.ComparisonGreater, nil
			default:
				return core.ComparisonEqual, nil
			}
		},
		Visit: func(sorted lineio.RecordRange) error {
			if observation.AfterBytes <= limit {
				return nil
			}
			if err := contextstate.Validate(ctx); err != nil {
				return err
			}
			var err error
			if left, err = readCacheTrimRecord(source, sorted, left); err != nil {
				return err
			}
			return evictCacheTrimRecord(request.Location.Root, left, &observation)
		},
	})
	if err != nil {
		return CacheTrimObservation{}, err
	}
	return observation, nil
}

func appendCacheTrimRecord(record []byte, path core.RelativePath, info fs.FileInfo, size uint64) ([]byte, error) {
	modified, err := temporal.NewInstant(info.ModTime())
	if err != nil {
		return nil, sourceError(err)
	}
	nanoseconds, err := modified.Nanoseconds()
	if err != nil {
		return nil, sourceError(err)
	}
	var field [8]byte
	modifiedField, err := binary.Append(field[:0], binary.BigEndian, nanoseconds)
	if err != nil {
		return nil, contractError(err)
	}
	// Flipping the two's-complement sign bit orders pre-1970 times first.
	modifiedField[0] ^= 0x80
	record = hex.AppendEncode(record, modifiedField)
	record = append(record, ' ')
	record = hex.AppendEncode(record, []byte(path.String()))
	record = append(record, ' ')
	record = hex.AppendEncode(record, binary.BigEndian.AppendUint64(field[:0], size))
	return append(record, lineio.Delimiter), nil
}

func readCacheTrimRecord(source lineio.RecordSource, record lineio.RecordRange, buffer []byte) ([]byte, error) {
	size, err := record.Bytes.Int64()
	if err != nil {
		return nil, contractError(err)
	}
	if int64(cap(buffer)) < size {
		buffer = make([]byte, size)
	}
	buffer = buffer[:size]
	if count, err := source.ReadAt(buffer, int64(record.Offset)); count != len(buffer) {
		return nil, sourceError(errors.Join(io.ErrUnexpectedEOF, err))
	}
	return buffer, nil
}

func evictCacheTrimRecord(root *os.Root, record []byte, observation *CacheTrimObservation) error {
	end := len(record) - 1
	if end < cacheTrimFixedBytes || record[end] != lineio.Delimiter ||
		record[cacheTrimFieldBytes] != ' ' || record[end-cacheTrimFieldBytes-1] != ' ' {
		return contractError(errors.New("filestore cache record is malformed"))
	}
	var field [8]byte
	if _, err := hex.Decode(field[:], record[end-cacheTrimFieldBytes:end]); err != nil {
		return contractError(err)
	}
	size := binary.BigEndian.Uint64(field[:])
	path, err := hex.AppendDecode(nil, record[cacheTrimFieldBytes+1:end-cacheTrimFieldBytes-1])
	if err != nil {
		return contractError(err)
	}
	err = root.Remove(string(path))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return cleanupError(err)
	}
	if err == nil {
		observation.RemovedFiles++
	}
	observation.AfterBytes -= min(size, observation.AfterBytes)
	return nil
}

var _ core.Validatable = CacheTrimRequest{}
