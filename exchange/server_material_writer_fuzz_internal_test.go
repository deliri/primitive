package exchange

import (
	"bytes"
	"context"
	json "encoding/json/v2"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

type materialWriterDoor uint8

const (
	materialWriteJSON materialWriterDoor = iota
	materialWriteNoBody
	materialWriteBounded
	materialWriteStream
	materialWriteSocket
)

type materialFuzzWriter struct {
	header                      http.Header
	body                        bytes.Buffer
	status, commits, writes     int
	fail                        bool
	acknowledge                 int
	panicWrite, panicAfterWrite bool
}

func (w *materialFuzzWriter) Header() http.Header    { return w.header }
func (w *materialFuzzWriter) WriteHeader(status int) { w.status = status; w.commits++ }
func (w *materialFuzzWriter) Write(data []byte) (int, error) {
	w.writes++
	if w.panicWrite {
		panic(io.ErrClosedPipe)
	}
	if w.fail {
		n := min(len(data), w.acknowledge)
		_, _ = w.body.Write(data[:n])
		return n, io.ErrClosedPipe
	}
	n, err := w.body.Write(data)
	if w.panicAfterWrite {
		panic(io.ErrClosedPipe)
	}
	return n, err
}

func FuzzWriteJSONNoBodyBoundedStreamAndSocketCustody(f *testing.F) {
	document := admissionJSONDocument{Message: "typed-fact"}
	if err := document.Validate(); err != nil {
		f.Fatal(err)
	}
	canonical, err := document.MarshalJSON()
	if err != nil {
		f.Fatal(err)
	}
	for _, extent := range []uint16{0, uint16(len(canonical) - 1), uint16(len(canonical)), uint16(len(canonical) + 1)} {
		f.Add(document.Message, canonical, extent, uint16(http.StatusOK), false, uint16(1), false)
	}
	for _, status := range []int{0, http.StatusContinue, http.StatusNoContent, http.StatusNotModified, http.StatusBadRequest, http.StatusInternalServerError} {
		f.Add(document.Message, canonical, uint16(len(canonical)), uint16(status), false, uint16(1), false)
	}
	f.Add(document.Message, canonical, uint16(len(canonical)), uint16(http.StatusOK), true, uint16(1), false)
	f.Add(document.Message, canonical, uint16(len(canonical)), uint16(http.StatusOK), false, uint16(1), true)
	f.Add("", []byte{}, uint16(0), uint16(http.StatusOK), true, uint16(0), false)
	f.Add(string([]byte{0xff}), []byte{0xff}, uint16(1), uint16(http.StatusOK), false, uint16(0), false)
	f.Fuzz(func(t *testing.T, text string, data []byte, extent, statusInput uint16, writeFailure bool, acknowledge uint16, cancelled bool) {
		if len(text) > 4096 || len(data) > 4096 {
			return
		}
		document := admissionJSONDocument{Message: text}
		encoded, encodeErr := json.Marshal(document)
		var status core.HTTPStatusCode
		statusErr := status.AdmitInt(int(statusInput))
		bodyPermitted := statusErr == nil && statusInput >= http.StatusOK && statusInput != http.StatusNoContent && statusInput != http.StatusNotModified
		var limit core.ByteCount
		if extent != 0 {
			limit = mustInternalByteCount(t, uint64(extent))
		}
		length, err := core.NewByteLength(uint64(extent))
		if err != nil {
			t.Fatal(err)
		}
		path, err := ParseSocketRoutePath("/socket")
		if err != nil {
			t.Fatal(err)
		}
		for _, door := range []materialWriterDoor{materialWriteJSON, materialWriteNoBody, materialWriteBounded, materialWriteStream, materialWriteSocket} {
			ctx, cancel := context.WithCancel(t.Context())
			if cancelled {
				cancel()
			}
			writer := &materialFuzzWriter{header: make(http.Header), fail: writeFailure, acknowledge: int(acknowledge)}
			source := bytes.NewReader(data)
			call, err := NewSocketServerCall(writer, httptest.NewRequestWithContext(ctx, http.MethodPost, path.String(), nil))
			if err != nil {
				cancel()
				t.Fatal(err)
			}
			var gotErr error
			socketRefused := false
			switch door {
			case materialWriteJSON:
				gotErr = WriteJSON(JSONWriteCall[admissionJSONDocument]{Call: call, Response: ServerJSONResponse[admissionJSONDocument]{Body: document, Status: status}, Policy: JSONWritePolicy{ResponseBodyLimit: limit}})
			case materialWriteNoBody:
				gotErr = WriteNoBody(NoBodyWriteCall{Call: call, Response: ServerNoBodyResponse{Status: status}})
			case materialWriteBounded:
				gotErr = WriteBounded(BoundedWriteCall{Call: call, Response: ServerBoundedResponse{Body: data, ContentType: core.HTTPMediaTypeOctetStream(), Status: status}})
			case materialWriteStream:
				gotErr = WriteStream(StreamWriteCall{Call: call, Response: ServerStreamResponse{Source: source, ContentType: core.HTTPMediaTypeOctetStream(), ContentLength: length, Status: status}})
			case materialWriteSocket:
				socket, constructorErr := NewServerSocket(JSONSocketContract{Path: path, Route: RouteSemantics{Method: MethodPost, Replay: ReplaySingleAttempt}, RequestBodyLimit: mustInternalByteCount(t, 1), ResponseBodyLimit: limit, SuccessStatus: status})
				socketRefused = extent == 0 || !bodyPermitted
				if socketRefused {
					if !errors.Is(constructorErr, core.ErrExchangeContract) || socket != (ServerSocket{}) {
						cancel()
						t.Fatalf("socket constructor=(%+v,%v), want zero and typed refusal", socket, constructorErr)
					}
				} else if constructorErr != nil {
					cancel()
					t.Fatal(constructorErr)
				}
				gotErr = WriteSocketJSON(socket, call, document)
			default:
				cancel()
				t.Fatalf("unclassified material writer %d", door)
			}
			cancel()
			admitted := bodyPermitted
			if door == materialWriteJSON || door == materialWriteSocket {
				admitted = admitted && extent > 0 && text != "" && encodeErr == nil && len(encoded) <= int(extent)
			}
			if door == materialWriteNoBody {
				admitted = statusErr == nil
			}
			admitted = admitted && !cancelled
			if !admitted {
				wantErr := core.ErrExchangeResponse
				if socketRefused {
					wantErr = core.ErrExchangeContract
				}
				if !errors.Is(gotErr, wantErr) || writer.commits != 0 || writer.writes != 0 || writer.body.Len() != 0 || len(writer.header) != 0 || source.Len() != len(data) {
					t.Fatalf("writer %d refusal=(%v,commits %d,writes %d,body %x,headers %v), want typed refusal before effects", door, gotErr, writer.commits, writer.writes, writer.body.Bytes(), writer.header)
				}
				if cancelled && !socketRefused && (!errors.Is(gotErr, context.Canceled) || !errors.Is(gotErr, core.ErrExchangeCancelled)) {
					t.Fatalf("writer %d lost cancellation: %v", door, gotErr)
				}
				continue
			}
			wantBody := data
			wantLength := len(data)
			wantType := core.HTTPMediaTypeOctetStream().String()
			wantWrites := 1
			wantRead := 0
			var wantErr error
			switch door {
			case materialWriteJSON, materialWriteSocket:
				wantBody = encoded
				wantLength = len(encoded)
				wantType = core.HTTPMediaTypeJSON().String()
			case materialWriteNoBody:
				wantBody = nil
				wantLength = 0
				wantType = ""
				wantWrites = 0
			case materialWriteStream:
				wantLength = int(extent)
				wantBody = data[:min(len(data), int(extent))]
				wantRead = min(len(data), int(extent)+1)
				if extent == 0 || len(data) == 0 {
					wantWrites = 0
				}
				if len(data) < int(extent) {
					wantErr = io.ErrUnexpectedEOF
				}
				if len(data) > int(extent) {
					wantErr = core.ErrExchangeBodyLimit
				}
			}
			if writeFailure && wantWrites > 0 {
				wantErr = io.ErrClosedPipe
				wantBody = wantBody[:min(len(wantBody), int(acknowledge))]
				if door == materialWriteStream {
					wantRead = min(len(data), int(extent))
				}
			}
			if !errors.Is(gotErr, wantErr) || wantErr != nil && (!errors.Is(gotErr, core.ErrExchangeResponse) || !errors.Is(gotErr, core.ErrExchangeWrite)) {
				t.Fatalf("writer %d error=%v, want native %v with response/write identity", door, gotErr, wantErr)
			}
			if writer.status != int(statusInput) || writer.commits != 1 || writer.writes != wantWrites || !bytes.Equal(writer.body.Bytes(), wantBody) || len(data)-source.Len() != wantRead {
				t.Fatalf("writer %d effects=status %d,commits %d,writes %d,body %x,read %d; want status %d,one commit,%d writes,%x,%d read", door, writer.status, writer.commits, writer.writes, writer.body.Bytes(), len(data)-source.Len(), statusInput, wantWrites, wantBody, wantRead)
			}
			wantFields := 2
			if door == materialWriteNoBody {
				wantFields = 1
			}
			if len(writer.header) != wantFields || writer.header.Get(core.HTTPHeaderContentType().String()) != wantType || writer.header.Get(core.HTTPHeaderContentLength().String()) != strconv.Itoa(wantLength) {
				t.Fatalf("writer %d headers=%v, want exact type %q and declaration %d", door, writer.header, wantType, wantLength)
			}
		}
	})
}
