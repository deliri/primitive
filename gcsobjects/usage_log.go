package gcsobjects

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"math"
	"net/netip"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/objectstore"
	"github.com/deliri/primitive/v2026/temporal"
)

// GCSUsageIPType is the provider's documented IP family, not a location.
type GCSUsageIPType uint8

const (
	GCSUsageIPUnknown GCSUsageIPType = iota
	GCSUsageIPv4
	GCSUsageIPv6
)

// GCSUsageMethod observes an HTTP token. Observing a custom method does not
// authorize Exchange to execute it.
type GCSUsageMethod string

func (m GCSUsageMethod) String() string { return string(m) }
func (m GCSUsageMethod) Validate() error {
	if m == "" {
		return core.ErrObjectStoreSource
	}
	for _, c := range m {
		if c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || strings.ContainsRune("!#$%&'*+-.^_`|~", c) {
			continue
		}
		return core.ErrObjectStoreSource
	}
	return nil
}

// GCSUsageRecord contains one provisional provider row. Empty Operation and
// zero Object represent the documented nullable columns. Other text is the
// actual observed string, including an empty referrer or user agent.
// The reserved c_ip_region column is deliberately not interpreted as geography.
type GCSUsageRecord struct {
	CompletedAt   temporal.Instant
	Latency       temporal.Duration
	ClientIP      netip.Addr
	ClientIPType  GCSUsageIPType
	Method        GCSUsageMethod
	Status        core.HTTPStatusCode
	RequestBytes  core.ByteLength
	ResponseBytes core.ByteLength
	Bucket        GCSBucket
	Object        GCSObjectName
	URI           string
	Host          string
	Referer       string
	UserAgent     string
	RequestID     string
	Operation     string
}

func (r GCSUsageRecord) Validate() error {
	if !r.ClientIP.IsValid() || !(r.ClientIPType == GCSUsageIPv4 && r.ClientIP.Is4() || r.ClientIPType == GCSUsageIPv6 && r.ClientIP.Is6()) {
		return core.ErrObjectStoreSource
	}
	if r.URI == "" || r.Host == "" || r.RequestID == "" {
		return core.ErrObjectStoreSource
	}
	for _, value := range [...]string{r.URI, r.Host, r.Referer, r.UserAgent, r.RequestID, r.Operation} {
		if !utf8.ValidString(value) {
			return core.ErrObjectStoreSource
		}
	}
	var objectErr error
	if r.Object != (GCSObjectName{}) {
		objectErr = r.Object.Validate()
	}
	return errors.Join(r.CompletedAt.Validate(), r.Latency.Validate(), r.Method.Validate(), r.Status.Validate(), r.RequestBytes.Validate(), r.ResponseBytes.Validate(), r.Bucket.Validate(), objectErr)
}

// GCSUsageLogRequest names one exact listed log generation and a caller-owned
// maximum extent. The bound also bounds CSV's largest in-memory record.
type GCSUsageLogRequest struct {
	Object  GCSObjectMetadata
	Maximum core.ByteCount
}

func (r GCSUsageLogRequest) Validate() error {
	if err := r.Object.Validate(); err != nil {
		return err
	}
	maximum, err := r.Maximum.Uint64()
	if err != nil || maximum > objectstore.GoogleCloudStorageObjectMaximumBytes || r.Object.Length().Uint64() > maximum {
		return errors.Join(core.ErrObjectStoreContract, core.ErrObjectStoreSize, err)
	}
	return nil
}

// GCSUsageVisitor receives rows in source order. Each row remains provisional
// until VisitGCSUsageLog returns its complete verified observation. Returning an
// error stops the scan and never releases a final observation.
type GCSUsageVisitor func(GCSUsageRecord) error

// GCSUsageLogObservation is constructed only after complete CSV, exact provider
// EOF/extent/CRC32C, caller lifetime and reader close have all succeeded.
type GCSUsageLogObservation struct {
	object  GCSObjectMetadata
	records uint64
}

func (o GCSUsageLogObservation) Validate() error { return o.object.Validate() }
func (o GCSUsageLogObservation) Metadata() (GCSObjectMetadata, error) {
	if err := o.Validate(); err != nil {
		return GCSObjectMetadata{}, err
	}
	return o.object, nil
}
func (o GCSUsageLogObservation) RecordCount() (uint64, error) {
	if err := o.Validate(); err != nil {
		return 0, err
	}
	return o.records, nil
}

// VisitGCSUsageLog uses the existing official SDK exact-generation reader and
// Go's CSV reader directly; it creates no producer goroutine, file or history.
func VisitGCSUsageLog(ctx context.Context, client *GCSClient, request GCSUsageLogRequest, visit GCSUsageVisitor) (GCSUsageLogObservation, error) {
	if err := errors.Join(validateGCSCall(ctx, client), request.Validate()); err != nil {
		return GCSUsageLogObservation{}, err
	}
	if visit == nil {
		return GCSUsageLogObservation{}, core.ErrObjectStoreDestination
	}
	reader, metadata, err := openGCSListedReader(ctx, client, request.Object)
	if err != nil {
		return GCSUsageLogObservation{}, err
	}
	extent, err := metadata.Length().Int64()
	if err != nil {
		return GCSUsageLogObservation{}, errors.Join(err, reader.Close())
	}
	checksum := crc32.New(crc32.MakeTable(crc32.Castagnoli))
	source := gcsReadSource{source: reader}
	remaining := io.LimitedReader{R: io.TeeReader(&source, checksum), N: extent}
	count, scanErr := visitGCSUsageCSV(ctx, &remaining, visit)
	var integrityErr error
	switch source.err {
	case nil, io.EOF:
	default:
		integrityErr = errors.Join(core.ErrObjectStoreSource, core.ErrObjectStoreIntegrity, source.err)
	}
	if scanErr == nil {
		if remaining.N != 0 {
			integrityErr = errors.Join(integrityErr, core.ErrObjectStoreIntegrity, io.ErrUnexpectedEOF)
		}
		integrityErr = errors.Join(integrityErr, proveGCSEOF(reader))
		if core.NewCRC32C(checksum.Sum32()) != metadata.CRC32C() {
			integrityErr = errors.Join(integrityErr, core.ErrObjectStoreIntegrity)
		}
	}
	if err := errors.Join(scanErr, integrityErr, reader.Close(), contextstate.Validate(ctx)); err != nil {
		return GCSUsageLogObservation{}, err
	}
	return GCSUsageLogObservation{object: metadata, records: count}, nil
}

type gcsUsageColumn uint8

const (
	usageTime gcsUsageColumn = iota
	usageIP
	usageIPType
	usageRegion
	usageMethod
	usageURI
	usageStatus
	usageRequestBytes
	usageResponseBytes
	usageLatency
	usageHost
	usageReferer
	usageUserAgent
	usageRequestID
	usageOperation
	usageBucket
	usageObject
	usageColumns
)

func usageColumnNames() [usageColumns]string {
	return [...]string{"time_micros", "c_ip", "c_ip_type", "c_ip_region", "cs_method", "cs_uri", "sc_status", "cs_bytes", "sc_bytes", "time_taken_micros", "cs_host", "cs_referer", "cs_user_agent", "s_request_id", "cs_operation", "cs_bucket", "cs_object"}
}

func visitGCSUsageCSV(ctx context.Context, source io.Reader, visit GCSUsageVisitor) (uint64, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return 0, err
	}
	reader := csv.NewReader(source)
	reader.ReuseRecord = true
	header, err := reader.Read()
	if err != nil {
		return 0, errors.Join(core.ErrObjectStoreSource, err)
	}
	var positions [usageColumns]int
	var seen [usageColumns]bool
	names := usageColumnNames()
	for index, name := range header {
		if name == "" || !utf8.ValidString(name) {
			return 0, core.ErrObjectStoreSource
		}
		for column, known := range names {
			if name != known {
				continue
			}
			if seen[column] {
				return 0, fmt.Errorf("%w: duplicated usage column %s", core.ErrObjectStoreSource, known)
			}
			positions[column], seen[column] = index, true
		}
	}
	for column, present := range seen {
		if !present {
			return 0, fmt.Errorf("%w: missing usage column %s", core.ErrObjectStoreSource, names[column])
		}
	}
	var count uint64
	for {
		if err := contextstate.Validate(ctx); err != nil {
			return 0, err
		}
		values, err := reader.Read()
		switch err {
		case io.EOF:
			return count, nil
		case nil:
		default:
			return 0, errors.Join(core.ErrObjectStoreSource, err)
		}
		for _, value := range values {
			if !utf8.ValidString(value) {
				return 0, core.ErrObjectStoreSource
			}
		}
		row, err := decodeGCSUsageRecord(values, positions)
		if err != nil {
			return 0, errors.Join(core.ErrObjectStoreSource, err)
		}
		if err := visit(row); err != nil {
			return 0, err
		}
		if err := contextstate.Validate(ctx); err != nil {
			return 0, err
		}
		count++
	}
}

func decodeGCSUsageRecord(values []string, positions [usageColumns]int) (GCSUsageRecord, error) {
	var row GCSUsageRecord
	value := func(column gcsUsageColumn) string { return values[positions[column]] }
	micros, err := strconv.ParseInt(value(usageTime), 10, 64)
	if err != nil || micros > math.MaxInt64/int64(temporal.NanosecondsPerMicrosecond) || micros < math.MinInt64/int64(temporal.NanosecondsPerMicrosecond) {
		return row, errors.Join(core.ErrObjectStoreSource, err, strconv.ErrRange)
	}
	row.CompletedAt = temporal.InstantFromNanoseconds(micros * int64(temporal.NanosecondsPerMicrosecond))
	row.ClientIP, err = netip.ParseAddr(value(usageIP))
	if err != nil {
		return GCSUsageRecord{}, err
	}
	family, err := strconv.ParseUint(value(usageIPType), 10, 8)
	if err != nil {
		return GCSUsageRecord{}, err
	}
	row.ClientIPType = GCSUsageIPType(family)
	status, err := strconv.ParseInt(value(usageStatus), 10, 32)
	if err != nil {
		return GCSUsageRecord{}, err
	}
	if err := row.Status.AdmitInt(int(status)); err != nil {
		return GCSUsageRecord{}, err
	}
	requestBytes, reqErr := strconv.ParseUint(value(usageRequestBytes), 10, 64)
	responseBytes, responseErr := strconv.ParseUint(value(usageResponseBytes), 10, 64)
	latency, latencyErr := strconv.ParseUint(value(usageLatency), 10, 64)
	if err := errors.Join(reqErr, responseErr, latencyErr); err != nil {
		return GCSUsageRecord{}, err
	}
	row.RequestBytes, reqErr = core.NewByteLength(requestBytes)
	row.ResponseBytes, responseErr = core.NewByteLength(responseBytes)
	row.Latency, latencyErr = temporal.DurationFromMicroseconds(latency)
	if err := errors.Join(reqErr, responseErr, latencyErr); err != nil {
		return GCSUsageRecord{}, err
	}
	row.Bucket, err = ParseGCSBucket(value(usageBucket))
	if err != nil {
		return GCSUsageRecord{}, err
	}
	if value(usageObject) != "" {
		row.Object, err = ParseGCSObjectName(value(usageObject))
		if err != nil {
			return GCSUsageRecord{}, err
		}
	}
	row.Method, row.URI, row.Host = GCSUsageMethod(value(usageMethod)), value(usageURI), value(usageHost)
	row.Referer, row.UserAgent, row.RequestID, row.Operation = value(usageReferer), value(usageUserAgent), value(usageRequestID), value(usageOperation)
	if err := row.Validate(); err != nil {
		return GCSUsageRecord{}, err
	}
	return row, nil
}
