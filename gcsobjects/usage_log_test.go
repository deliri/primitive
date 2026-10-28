package gcsobjects

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"math"
	"net/http"
	"net/netip"
	"slices"
	"strconv"
	"sync/atomic"
	"testing"

	storageapi "google.golang.org/api/storage/v1"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

func usageRecordFixture(t testing.TB) GCSUsageRecord {
	t.Helper()
	var status core.HTTPStatusCode
	if err := status.AdmitInt(http.StatusOK); err != nil {
		t.Fatal(err)
	}
	latency, err := temporal.DurationFromMicroseconds(1200)
	if err != nil {
		t.Fatal(err)
	}
	row := GCSUsageRecord{CompletedAt: temporal.InstantFromNanoseconds(gcsProviderInstantNanos), Latency: latency,
		ClientIP: netip.MustParseAddr("192.0.2.1"), ClientIPType: GCSUsageIPv4, Method: "GET", Status: status,
		Bucket: parsedGCSBucket(t, gcsProviderBucketText), Object: parsedGCSObjectName(t, "logo.svg"),
		URI: "/logo.svg", Host: "storage.googleapis.com", UserAgent: "Usage fixture", RequestID: "fixture-request", Operation: "GET_Object"}
	row.RequestBytes, err = core.NewByteLength(0)
	if err != nil {
		t.Fatal(err)
	}
	row.ResponseBytes, err = core.NewByteLength(41)
	if err != nil {
		t.Fatal(err)
	}
	if err := row.Validate(); err != nil {
		t.Fatal(err)
	}
	return row
}

func usageRecordValues(t testing.TB, row GCSUsageRecord) []string {
	t.Helper()
	if err := row.Validate(); err != nil {
		t.Fatal(err)
	}
	nanos, err := row.CompletedAt.Nanoseconds()
	if err != nil {
		t.Fatal(err)
	}
	status, err := row.Status.Int()
	if err != nil {
		t.Fatal(err)
	}
	return []string{strconv.FormatInt(nanos/int64(temporal.NanosecondsPerMicrosecond), 10), row.ClientIP.String(), strconv.FormatUint(uint64(row.ClientIPType), 10), "", row.Method.String(), row.URI, strconv.Itoa(status), strconv.FormatUint(row.RequestBytes.Uint64(), 10), strconv.FormatUint(row.ResponseBytes.Uint64(), 10), strconv.FormatInt(row.Latency.Nanoseconds()/int64(temporal.NanosecondsPerMicrosecond), 10), row.Host, row.Referer, row.UserAgent, row.RequestID, row.Operation, row.Bucket.String(), row.Object.String()}
}

func usageCSVFixture(t testing.TB, header []string, rows ...[]string) []byte {
	t.Helper()
	var b bytes.Buffer
	w := csv.NewWriter(&b)
	if err := w.Write(header); err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if err := w.Write(row); err != nil {
			t.Fatal(err)
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// Each row changes a load-bearing provider fact. This is CSV/provider admission,
// not a separate policy classifier or a claim about production log delivery.
func TestGCSUsageCSVBoundary(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		column  gcsUsageColumn
		value   string
		wantErr bool
	}{
		{name: "zero timestamp is the actual Unix epoch", column: usageTime, value: "0"},
		{name: "negative timestamp remains signed time", column: usageTime, value: "-1"},
		{name: "last representable microsecond", column: usageTime, value: strconv.FormatInt(math.MaxInt64/1000, 10)},
		{name: "first representable microsecond", column: usageTime, value: strconv.FormatInt(math.MinInt64/1000, 10)},
		{name: "timestamp above nanosecond range", column: usageTime, value: strconv.FormatInt(math.MaxInt64/1000+1, 10), wantErr: true},
		{name: "timestamp below nanosecond range", column: usageTime, value: strconv.FormatInt(math.MinInt64/1000-1, 10), wantErr: true},
		{name: "timestamp decimal is not an integer", column: usageTime, value: "1.5", wantErr: true},
		{name: "IPv4 address remains typed", column: usageIP, value: "198.51.100.19"},
		{name: "IPv6 cannot claim IPv4 family", column: usageIP, value: "2001:db8::1", wantErr: true},
		{name: "nonaddress text is refused", column: usageIP, value: "not-an-address", wantErr: true},
		{name: "unknown IP family is refused", column: usageIPType, value: "0", wantErr: true},
		{name: "future IP family is refused", column: usageIPType, value: "3", wantErr: true},
		{name: "oversized IP family cannot wrap", column: usageIPType, value: "257", wantErr: true},
		{name: "observed TRACE does not authorize execution", column: usageMethod, value: "TRACE"},
		{name: "observed extension method stays a token", column: usageMethod, value: "PROPFIND"},
		{name: "space is not an HTTP method token", column: usageMethod, value: "BAD METHOD", wantErr: true},
		{name: "empty method has no typed meaning", column: usageMethod, value: "", wantErr: true},
		{name: "status below HTTP domain", column: usageStatus, value: "99", wantErr: true},
		{name: "first HTTP status", column: usageStatus, value: "100"},
		{name: "last HTTP status", column: usageStatus, value: "599"},
		{name: "status above HTTP domain", column: usageStatus, value: "600", wantErr: true},
		{name: "oversized status cannot wrap", column: usageStatus, value: "4294967496", wantErr: true},
		{name: "zero response bytes remain observed zero", column: usageResponseBytes, value: "0"},
		{name: "maximum signed response extent", column: usageResponseBytes, value: strconv.FormatInt(math.MaxInt64, 10)},
		{name: "response extent above signed domain", column: usageResponseBytes, value: "9223372036854775808", wantErr: true},
		{name: "negative response bytes cannot become zero", column: usageResponseBytes, value: "-1", wantErr: true},
		{name: "negative request bytes cannot become zero", column: usageRequestBytes, value: "-1", wantErr: true},
		{name: "unsigned counter overflow refuses", column: usageRequestBytes, value: "18446744073709551616", wantErr: true},
		{name: "zero latency remains observed zero", column: usageLatency, value: "0"},
		{name: "last representable latency microsecond", column: usageLatency, value: strconv.FormatInt(math.MaxInt64/1000, 10)},
		{name: "latency above duration range", column: usageLatency, value: strconv.FormatInt(math.MaxInt64/1000+1, 10), wantErr: true},
		{name: "negative latency cannot become zero", column: usageLatency, value: "-1", wantErr: true},
		{name: "quoted comma in observed user agent", column: usageUserAgent, value: "client, device"},
		{name: "quoted quote in observed user agent", column: usageUserAgent, value: `client "version"`},
		{name: "quoted newline remains observed text", column: usageUserAgent, value: "client\nsecond line"},
		{name: "Unicode text is not normalized", column: usageUserAgent, value: "Café e\u0301"},
		{name: "empty referrer is observed empty text", column: usageReferer, value: ""},
		{name: "nullable operation is absent", column: usageOperation, value: ""},
		{name: "nullable object is absent", column: usageObject, value: ""},
		{name: "reserved region cannot mint geography", column: usageRegion, value: "future-reserved-text"},
		{name: "invalid bucket cannot cross the provider boundary", column: usageBucket, value: "INVALID BUCKET", wantErr: true},
		{name: "empty request identifier is refused", column: usageRequestID, value: "", wantErr: true},
		{name: "empty request URI is refused", column: usageURI, value: "", wantErr: true},
		{name: "empty original host is refused", column: usageHost, value: "", wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			seed := usageRecordFixture(t)
			values := usageRecordValues(t, seed)
			values[tc.column] = tc.value
			names := usageColumnNames()
			wire := usageCSVFixture(t, names[:], values)
			var got GCSUsageRecord
			visits := 0
			count, err := visitGCSUsageCSV(t.Context(), bytes.NewReader(wire), func(row GCSUsageRecord) error { got = row; visits++; return nil })
			if tc.wantErr {
				if !errors.Is(err, core.ErrObjectStoreSource) || count != 0 || visits != 0 {
					t.Fatalf("CSV admission = %v/%d/%d, want typed refusal and zero row/count", err, count, visits)
				}
				return
			}
			if err != nil || count != 1 || visits != 1 || got.Validate() != nil {
				t.Fatalf("CSV admission = %v/%d/%d/%v, want one validated row", err, count, visits, got)
			}
			if got.CompletedAt != seed.CompletedAt && tc.column != usageTime || got.Status != seed.Status && tc.column != usageStatus {
				t.Fatalf("unrelated typed facts changed = %+v, want preserved seed", got)
			}
			if tc.column == usageUserAgent && got.UserAgent != tc.value {
				t.Fatalf("observed user agent = %q, want exact %q", got.UserAgent, tc.value)
			}
		})
	}
}

func TestGCSUsageCSVFramingLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		kind     int
		wantErr  error
		wantRows uint64
	}{
		{name: "complete typed row", wantRows: 1},
		{name: "header only is an honest empty log", kind: 1},
		{name: "known column order is not positional", kind: 2, wantRows: 1},
		{name: "future optional column can precede known columns", kind: 3, wantRows: 1},
		{name: "missing known column refuses", kind: 4, wantErr: core.ErrObjectStoreSource},
		{name: "duplicate known column refuses even identical values", kind: 5, wantErr: core.ErrObjectStoreSource},
		{name: "row count mismatch preserves CSV parse error", kind: 6, wantErr: core.ErrObjectStoreSource},
		{name: "unclosed quoted record preserves CSV parse error", kind: 7, wantErr: core.ErrObjectStoreSource},
		{name: "invalid UTF8 in unowned column refuses", kind: 8, wantErr: core.ErrObjectStoreSource},
		{name: "no header cannot prove an empty log", kind: 9, wantErr: core.ErrObjectStoreSource},
		{name: "IPv6 uses its actual documented family", kind: 10, wantRows: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			seed := usageRecordFixture(t)
			names := usageColumnNames()
			header := slices.Clone(names[:])
			values := usageRecordValues(t, seed)
			rows := [][]string{values}
			switch tc.kind {
			case 0:
			case 1:
				rows = nil
			case 2:
				slices.Reverse(header)
				slices.Reverse(values)
			case 3:
				header = append([]string{"future_optional"}, header...)
				rows = [][]string{append([]string{"opaque"}, values...)}
			case 4:
				header = header[:len(header)-1]
				rows = [][]string{values[:len(values)-1]}
			case 5:
				header = append(header, header[0])
				rows = [][]string{append(values, values[0])}
			case 6:
				rows = [][]string{values[:len(values)-1]}
			case 7:
			case 8:
				header = append(header, "future_optional")
				rows = [][]string{append(values, string([]byte{0xff}))}
			case 9:
			case 10:
				values[usageIP] = "2001:db8::1"
				values[usageIPType] = "2"
			default:
				t.Fatal("unknown fixture mutation")
			}
			wire := usageCSVFixture(t, header, rows...)
			if tc.kind == 7 {
				wire = append(usageCSVFixture(t, header), []byte("\"unterminated")...)
			}
			if tc.kind == 9 {
				wire = nil
			}
			var got GCSUsageRecord
			visits := uint64(0)
			count, err := visitGCSUsageCSV(t.Context(), bytes.NewReader(wire), func(row GCSUsageRecord) error { got = row; visits++; return nil })
			if !errors.Is(err, tc.wantErr) || count != tc.wantRows || visits != tc.wantRows {
				t.Fatalf("framing = %v/%d/%d, want %v/%d", err, count, visits, tc.wantErr, tc.wantRows)
			}
			if tc.kind == 6 || tc.kind == 7 {
				var parse *csv.ParseError
				if !errors.As(err, &parse) {
					t.Fatalf("CSV error=%v, want retained *csv.ParseError", err)
				}
			}
			if err == nil && tc.wantRows == 1 {
				if tc.kind == 10 {
					seed.ClientIP = netip.MustParseAddr("2001:db8::1")
					seed.ClientIPType = GCSUsageIPv6
				}
				if got != seed {
					t.Fatalf("row=%+v,want exact typed %+v", got, seed)
				}
			}
		})
	}
}

func TestGCSUsageCSVFailureOwnershipLayerTriad(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"caller cancelled before header", "visitor refuses after one row", "visitor cancels caller after row", "source error preserves native cause"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			names := usageColumnNames()
			values := usageRecordValues(t, usageRecordFixture(t))
			wire := usageCSVFixture(t, names[:], values)
			ctx, cancel := context.WithCancelCause(t.Context())
			defer cancel(nil)
			cause := fmt.Errorf("fixture cancellation: %w", io.ErrClosedPipe)
			var source io.Reader = bytes.NewReader(wire)
			visits := 0
			wantVisits := 1
			switch kind {
			case "caller cancelled before header":
				cancel(cause)
				wantVisits = 0
			case "visitor refuses after one row":
			case "visitor cancels caller after row":
			case "source error preserves native cause":
				source = io.MultiReader(bytes.NewReader(usageCSVFixture(t, names[:])), usageErrorReader{cause})
				wantVisits = 0
			default:
				t.Fatal("unknown failure fixture")
			}
			count, err := visitGCSUsageCSV(ctx, source, func(GCSUsageRecord) error {
				visits++
				if kind == "visitor refuses after one row" {
					return cause
				}
				cancel(cause)
				return nil
			})
			if !errors.Is(err, cause) || count != 0 || visits != wantVisits {
				t.Fatalf("failure ownership=%v/%d/%d, want original cause/zero/%d", err, count, visits, wantVisits)
			}
		})
	}
}

type usageErrorReader struct{ err error }

func (r usageErrorReader) Read([]byte) (int, error) { return 0, r.err }

// Real local HTTP and the official SDK exercise exact generation, metadata,
// body extent/checksum and provisional rows; no deployed provider is contacted.
func TestVisitGCSUsageLogTerminalLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name       string
		kind       int
		wantErr    error
		wantRows   uint64
		wantVisits uint64
	}{
		{name: "complete read releases exact terminal observation", wantRows: 1, wantVisits: 1},
		{name: "header-only log is complete without invented rows", kind: 1},
		{name: "metadata checksum disagreement cannot seal visited row", kind: 2, wantErr: core.ErrObjectStoreIntegrity, wantVisits: 1},
		{name: "short provider body cannot seal visited row", kind: 3, wantErr: core.ErrObjectStoreIntegrity, wantVisits: 1},
		{name: "extra provider bytes cannot seal visited row", kind: 4, wantErr: core.ErrObjectStoreIntegrity, wantVisits: 1},
		{name: "foreign metadata generation cannot seal listed object", kind: 5, wantErr: core.ErrObjectStoreConflict},
		{name: "caller maximum refuses before provider read", kind: 6, wantErr: core.ErrObjectStoreSize},
		{name: "missing visitor refuses before provider read", kind: 7, wantErr: core.ErrObjectStoreDestination},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			names := usageColumnNames()
			data := usageCSVFixture(t, names[:], usageRecordValues(t, usageRecordFixture(t)))
			if tc.kind == 1 {
				data = usageCSVFixture(t, names[:])
			}
			var calls, media atomic.Int32
			// The channel publishes an immutable listed fact after the actual SDK lookup.
			// No handler waits on it during lookup; every subsequent response uses it.
			expected := make(chan GCSObjectMetadata, 1)
			client := bucketTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if r.URL.Query().Get("alt") == "json" {
					select {
					case listed := <-expected:
						expected <- listed
						if tc.kind == 5 {
							listed.generation, _ = NewGCSGeneration(gcsProviderGeneration + 1)
						}
						writeUsageListedResponse(t, w, listed)
					default:
						writeGCSAttrsResponse(t, w, data)
					}
					return
				}
				media.Add(1)
				if got := r.URL.Query().Get("generation"); got != strconv.FormatInt(gcsProviderGeneration, 10) {
					t.Errorf("read generation=%q,want exact listed", got)
				}
				body := data
				if tc.kind == 4 {
					body = append(bytes.Clone(data), 'x')
				}
				writeGCSMediaResponseFraming(t, w, body, false)
			}))
			listed, err := LookupGCSObject(t.Context(), client, GCSObjectLookupRequest{Bucket: parsedGCSBucket(t, gcsProviderBucketText), Name: parsedGCSObjectName(t, gcsProviderObjectText)})
			if err != nil {
				t.Fatal(err)
			}
			if tc.kind == 2 {
				checksum, crcErr := listed.CRC32C().Uint32()
				if crcErr != nil {
					t.Fatal(crcErr)
				}
				listed.crc32c = core.NewCRC32C(checksum ^ 1)
			}
			if tc.kind == 3 {
				listed.length, err = core.NewByteLength(uint64(len(data) + 1))
				if err != nil {
					t.Fatal(err)
				}
			}
			expected <- listed
			priorCalls := calls.Load()
			maximum := gcsProviderMaximum(t, uint64(len(data)+8))
			if tc.kind == 6 {
				maximum = gcsProviderMaximum(t, uint64(len(data)-1))
			}
			visits := uint64(0)
			visitor := GCSUsageVisitor(func(row GCSUsageRecord) error { visits++; return row.Validate() })
			if tc.kind == 7 {
				visitor = nil
			}
			got, err := VisitGCSUsageLog(t.Context(), client, GCSUsageLogRequest{Object: listed, Maximum: maximum}, visitor)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) || got != (GCSUsageLogObservation{}) || visits != tc.wantVisits {
					t.Fatalf("terminal refusal=%v/%v visits=%d,want zero/%v visits=%d", got, err, visits, tc.wantErr, tc.wantVisits)
				}
				if tc.kind == 6 || tc.kind == 7 {
					if calls.Load() != priorCalls || media.Load() != 0 {
						t.Fatalf("refused request made %d provider calls/%d media,want zero", calls.Load()-priorCalls, media.Load())
					}
				}
				return
			}
			count, countErr := got.RecordCount()
			metadata, metaErr := got.Metadata()
			if err != nil || countErr != nil || metaErr != nil || metadata != listed || count != tc.wantRows || visits != tc.wantRows || media.Load() != 1 {
				t.Fatalf("terminal=%v/%v count=%d visits=%d media=%d,want exact metadata/count%d", got, err, count, visits, media.Load(), tc.wantRows)
			}
		})
	}
}

func writeUsageListedResponse(t testing.TB, w http.ResponseWriter, listed GCSObjectMetadata) {
	t.Helper()
	generation, err := listed.Generation().Int64()
	if err != nil {
		t.Error(err)
		return
	}
	checksum, err := listed.CRC32C().Base64()
	if err != nil {
		t.Error(err)
		return
	}
	instant, err := temporal.InstantFromNanoseconds(gcsProviderInstantNanos).RFC3339Nano()
	if err != nil {
		t.Error(err)
		return
	}
	writeProviderJSON(t, w, storageapi.Object{Bucket: listed.Bucket().String(), Name: listed.Name().String(), Generation: generation, Size: listed.Length().Uint64(), Crc32c: checksum, ContentType: gcsProviderMediaTypeText, CacheControl: gcsProviderCacheControlText, TimeCreated: instant, Updated: instant, CustomTime: instant})
}

// Raw CSV reaches the public official-SDK read door. The independent standard
// CSV reader binds each emitted fact back to its actual source field. Canonical
// re-encoding/decoding then proves typed closure rather than panic survival.
func FuzzVisitGCSUsageLogSemanticClosure(f *testing.F) {
	seed := usageRecordFixture(f)
	names := usageColumnNames()
	values := usageRecordValues(f, seed)
	canonical := usageCSVFixture(f, names[:], values)
	f.Add(canonical)
	f.Add(usageCSVFixture(f, names[:]))
	f.Add([]byte{})
	f.Add(canonical[:len(canonical)-3])
	duplicate := slices.Clone(names[:])
	duplicate[len(duplicate)-1] = duplicate[0]
	f.Add(usageCSVFixture(f, duplicate, values))
	f.Add(usageCSVFixture(f, append([]string{"future_nullable"}, names[:]...), append([]string{""}, values...)))
	f.Fuzz(func(t *testing.T, data []byte) {
		const maximum = 64 * 1024
		// Build immutable provider facts using the actual typed SDK fixture and
		// official lookup, never an opaque success/result stub.
		initial := bucketTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { writeGCSAttrsResponse(t, w, canonical) }))
		listed, err := LookupGCSObject(t.Context(), initial, GCSObjectLookupRequest{Bucket: seed.Bucket, Name: parsedGCSObjectName(t, gcsProviderObjectText)})
		if err != nil {
			t.Fatal(err)
		}
		listed.length, err = core.NewByteLength(uint64(len(data)))
		if err != nil {
			t.Fatal(err)
		}
		listed.crc32c = core.NewCRC32C(crc32.Checksum(data, crc32.MakeTable(crc32.Castagnoli)))
		var calls atomic.Int32
		client := bucketTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls.Add(1)
			if r.URL.Query().Get("alt") == "json" {
				writeUsageListedResponse(t, w, listed)
				return
			}
			writeGCSMediaResponseFraming(t, w, data, false)
		}))
		reference := csv.NewReader(bytes.NewReader(data))
		header, headerErr := reference.Read()
		visited := uint64(0)
		got, err := VisitGCSUsageLog(t.Context(), client, GCSUsageLogRequest{Object: listed, Maximum: gcsProviderMaximum(t, maximum)}, func(row GCSUsageRecord) error {
			if headerErr != nil {
				t.Fatalf("emitted row despite reference header error=%v", headerErr)
			}
			raw, readErr := reference.Read()
			if readErr != nil {
				t.Fatalf("emitted row despite reference CSV error=%v", readErr)
			}
			usageOracleSourceFacts(t, row, header, raw)
			encoded := usageCSVFixture(t, names[:], usageRecordValues(t, row))
			var round GCSUsageRecord
			count, roundErr := visitGCSUsageCSV(t.Context(), bytes.NewReader(encoded), func(value GCSUsageRecord) error { round = value; return nil })
			if roundErr != nil || count != 1 || round != row {
				t.Fatalf("typed closure=%+v/%d/%v,want exact %+v/1/nil", round, count, roundErr, row)
			}
			second := usageCSVFixture(t, names[:], usageRecordValues(t, round))
			if !bytes.Equal(second, encoded) {
				t.Fatal("second CSV write differs from canonical first write")
			}
			visited++
			return nil
		})
		if len(data) > maximum {
			if !errors.Is(err, core.ErrObjectStoreSize) || got != (GCSUsageLogObservation{}) || visited != 0 || calls.Load() != 0 {
				t.Fatalf("over-bound result=%v/%v/%d/%d,want zero typed size refusal", got, err, visited, calls.Load())
			}
			return
		}
		if err != nil {
			if !errors.Is(err, core.ErrObjectStoreSource) || got != (GCSUsageLogObservation{}) {
				t.Fatalf("CSV refusal=%v/%v,want typed source refusal and zero final observation", got, err)
			}
			return
		}
		tail, tailErr := reference.Read()
		count, countErr := got.RecordCount()
		metadata, metaErr := got.Metadata()
		if headerErr != nil || tailErr != io.EOF || len(tail) != 0 || countErr != nil || metaErr != nil || metadata != listed || count != visited {
			t.Fatalf("complete source/count closure=%v/%v rows=%d visits=%d metadata=%v/%v,want exact EOF/listed/count", headerErr, tailErr, count, visited, metadata, metaErr)
		}
	})
}

func usageOracleSourceFacts(t testing.TB, row GCSUsageRecord, header, raw []string) {
	t.Helper()
	if err := row.Validate(); err != nil {
		t.Fatalf("emitted row.Validate=%v,want nil", err)
	}
	names := usageColumnNames()
	canonical := usageRecordValues(t, row)
	for column, name := range names {
		if gcsUsageColumn(column) == usageRegion {
			continue
		}
		position := -1
		for index, field := range header {
			if field == name {
				if position != -1 {
					t.Fatalf("emitted row from duplicate %s", name)
				}
				position = index
			}
		}
		if position < 0 || position >= len(raw) {
			t.Fatalf("emitted row without source field %s", name)
		}
		source, value := raw[position], canonical[column]
		switch gcsUsageColumn(column) {
		case usageTime, usageStatus:
			sourceInt, sourceErr := strconv.ParseInt(source, 10, 64)
			valueInt, valueErr := strconv.ParseInt(value, 10, 64)
			if sourceErr != nil || valueErr != nil || sourceInt != valueInt {
				t.Fatalf("%s source=%q projected=%q,want exact signed quantity", name, source, value)
			}
		case usageIPType, usageRequestBytes, usageResponseBytes, usageLatency:
			sourceInt, sourceErr := strconv.ParseUint(source, 10, 64)
			valueInt, valueErr := strconv.ParseUint(value, 10, 64)
			if sourceErr != nil || valueErr != nil || sourceInt != valueInt {
				t.Fatalf("%s source=%q projected=%q,want exact unsigned quantity", name, source, value)
			}
		case usageIP:
			sourceIP, sourceErr := netip.ParseAddr(source)
			if sourceErr != nil || sourceIP != row.ClientIP {
				t.Fatalf("IP source=%q projected=%s,want actual source address", source, row.ClientIP)
			}
		default:
			if source != value {
				t.Fatalf("%s source=%q projected=%q,want exact source text", name, source, value)
			}
		}
	}
}
