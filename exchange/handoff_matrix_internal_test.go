package exchange

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"slices"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

type handoffPrimary uint8

const (
	handoffContradiction handoffPrimary = iota
	handoffRefusal
	handoffNeutral
	handoffBoundary
)

// The status decision domain is core.HTTPStatusCode's entire uint16 carrier,
// filtered through its real admission boundary. No list of currently familiar
// status codes defines the domain. Every admitted status reaches the real
// producer and both classification paths, with exact expected status binding.
func TestHTTPProducerClassifierStatusDomainLayerTriad(t *testing.T) {
	t.Parallel()
	modes := []struct {
		name      string
		cancel    bool
		replay    ReplayMode
		remaining uint64
	}{
		{name: "single attempt forbids status-driven replay", replay: ReplaySingleAttempt, remaining: 1},
		{name: "last available retry is usable", replay: ReplaySafe, remaining: 1},
		{name: "zero remaining retries exhausts without inventing another effect", replay: ReplaySafe},
		{name: "maximum retry counter cannot wrap into exhaustion", replay: ReplaySafe, remaining: math.MaxUint64},
		{name: "cancellation after the producer forbids a new effect", replay: ReplaySafe, remaining: 1, cancel: true},
	}
	statuses := make([]core.HTTPStatusCode, 0)
	for raw := 0; raw <= math.MaxUint16; raw++ {
		var status core.HTTPStatusCode
		if status.AdmitInt(raw) == nil {
			statuses = append(statuses, status)
		}
	}
	if len(statuses) == 0 {
		t.Fatalf("core status domain=%v, want nonempty", statuses)
	}
	for _, mode := range modes {
		t.Run(mode.name, func(t *testing.T) {
			t.Parallel()
			for _, status := range statuses {
				raw, err := status.Int()
				if err != nil {
					t.Fatal(err)
				}
				foreign := core.HTTPStatusOK()
				if foreign == status {
					if err := foreign.AdmitInt(http.StatusCreated); err != nil {
						t.Fatal(err)
					}
				}
				for _, expected := range []core.HTTPStatusCode{foreign, status} {
					wantRaw, err := expected.Int()
					if err != nil {
						t.Fatal(err)
					}
					retryStatus := raw >= http.StatusInternalServerError || slices.Contains([]int{http.StatusRequestTimeout, http.StatusTooEarly, http.StatusTooManyRequests}, raw)
					primary := handoffContradiction
					if status == expected {
						primary = handoffNeutral
					}
					if status != expected && slices.Contains([]int{http.StatusRequestTimeout - 1, http.StatusRequestTimeout, http.StatusRequestTimeout + 1, http.StatusInternalServerError - 1, http.StatusInternalServerError}, raw) {
						primary = handoffBoundary
					}
					row := struct {
						status, expected                                                 core.HTTPStatusCode
						primary                                                          handoffPrimary
						wantAggregate                                                    attemptDisposition
						wantAggregateError, wantStreamError                              error
						wantAggregateStatusError, wantStreamStatusError, wantStreamRetry bool
					}{status: status, expected: expected, primary: primary, wantAggregate: attemptComplete}
					if status != expected {
						row.wantAggregateError = core.ErrExchangeResponse
						row.wantAggregateStatusError = true
						row.wantStreamError = core.ErrExchangeResponse
						row.wantStreamStatusError = true
						row.wantStreamRetry = retryStatus
						if retryStatus && mode.replay == ReplaySafe {
							row.wantAggregate = attemptExhausted
							if mode.remaining > 0 {
								row.wantAggregate = attemptRetry
								row.wantAggregateError = nil
								row.wantAggregateStatusError = false
							}
						}
					}
					if mode.cancel {
						row.wantAggregate = attemptComplete
						row.wantAggregateError = context.Canceled
						row.wantAggregateStatusError = false
						row.wantStreamRetry = false
						row.wantStreamError = context.Canceled
					}
					t.Run(fmt.Sprintf("status_%d_expected_%d_primary_%d", raw, wantRaw, row.primary), func(t *testing.T) {
						t.Parallel()
						target, err := core.ParseHTTPEndpoint("https://provider.example.test/status-domain")
						if err != nil {
							t.Fatal(err)
						}
						limit, err := core.NewByteCount(2)
						if err != nil {
							t.Fatal(err)
						}
						semantics := RequestSemantics{Method: MethodGet, Replay: mode.replay}
						if err := semantics.Validate(); err != nil {
							t.Fatal(err)
						}
						for _, stream := range []bool{false, true} {
							if stream && (mode.replay != ReplaySafe || mode.remaining != 1) {
								continue
							}
							ctx, cancel := context.WithCancel(t.Context())
							body := &replayHandoffBody{reader: bytes.NewReader(nil)}
							calls := 0
							goClient := &http.Client{Transport: replayHandoffTransport(func(request *http.Request) (*http.Response, error) {
								calls++
								return &http.Response{StatusCode: raw, ContentLength: 0, Body: body, Request: request}, nil
							})}
							if stream {
								client, err := NewClient(goClient)
								if err != nil {
									cancel()
									t.Fatal(err)
								}
								var destination bytes.Buffer
								produced, producerErr := Download(DownloadCall{Context: ctx, Client: client, Request: DownloadRequest{Target: target, Destination: &destination, ExpectedStatus: row.expected, ResponseBodyLimit: limit, Semantics: RequestSemantics{Method: MethodGet, Replay: ReplaySingleAttempt}}, Policy: StreamPolicy{OperationTimeout: runtimeAgreementPolicy(t).ReadTimeout, AttemptTimeout: runtimeAgreementPolicy(t).ReadTimeout, ErrorBodyLimit: limit, Redirect: RedirectPolicy{Mode: RedirectReject}}})
								if produced.Metadata.Status != row.status || produced.Metadata.Attempts != 1 || produced.Metadata.Bytes != (core.ByteLength{}) || produced.DeclaredRequestBytes != (core.ByteLength{}) || produced.Metadata.Headers.Values != nil || destination.Len() != 0 {
									cancel()
									t.Fatalf("stream producer = %+v, want exact status %v and no byte/header evidence", produced, row.status)
								}
								statusErr, hasStatus := errors.AsType[StatusError](producerErr)
								if hasStatus != (status != expected) || hasStatus && (statusErr.Status() != row.status || statusErr.Expected() != row.expected) || (producerErr == nil) != (status == expected) {
									cancel()
									t.Fatalf("stream producer error = %v, want exact observed/required status binding", producerErr)
								}
								retained := produced
								if mode.cancel {
									cancel()
								}
								admitted, admissionErr := admitStreamAttemptResponse(produced, producerErr, 1)
								retry, gotErr := replayStreamDecision(ctx, admissionErr)
								if retry != row.wantStreamRetry || !errors.Is(gotErr, row.wantStreamError) {
									cancel()
									t.Fatalf("stream classifier = %t/%v, want %t/%v", retry, gotErr, row.wantStreamRetry, row.wantStreamError)
								}
								_, hasStatus = errors.AsType[StatusError](gotErr)
								if hasStatus != row.wantStreamStatusError {
									cancel()
									t.Fatalf("stream retained status error = %t, want %t", hasStatus, row.wantStreamStatusError)
								}
								if admitted.Metadata.Status != retained.Metadata.Status || admitted.Metadata.Bytes != retained.Metadata.Bytes || admitted.Metadata.Attempts != retained.Metadata.Attempts || admitted.Metadata.Headers.Values != nil || admitted.DeclaredRequestBytes != retained.DeclaredRequestBytes {
									cancel()
									t.Fatalf("stream classifier changed producer evidence: %+v -> %+v", retained, admitted)
								}
							} else {
								produced, producerErr := executeAggregateAttempt(aggregateAttempt{context: ctx, client: goClient, request: aggregateRequest{target: target, semantics: semantics, expectedStatus: row.expected}, timeout: runtimeAgreementPolicy(t).ReadTimeout, limit: limit})
								if producerErr != nil || produced.status != row.status || produced.body != nil || produced.headers.Values != nil || produced.retryAfter != "" {
									cancel()
									t.Fatalf("aggregate producer = (%+v,%v), want status %v with no body/header/hint evidence", produced, producerErr, row.status)
								}
								if mode.cancel {
									cancel()
								}
								class, gotErr := classifyAggregateAttempt(aggregateAttemptResult{operationContext: ctx, cause: producerErr, semantics: semantics, response: produced, expected: row.expected, attemptsRemaining: mode.remaining})
								if class != row.wantAggregate || !errors.Is(gotErr, row.wantAggregateError) {
									cancel()
									t.Fatalf("aggregate classifier = %v/%v, want %v/%v", class, gotErr, row.wantAggregate, row.wantAggregateError)
								}
								statusErr, hasStatus := errors.AsType[StatusError](gotErr)
								if hasStatus != row.wantAggregateStatusError || hasStatus && (statusErr.Status() != row.status || statusErr.Expected() != row.expected) {
									cancel()
									t.Fatalf("aggregate retained status error = %v, want present=%t and exact binding", gotErr, row.wantAggregateStatusError)
								}
								if produced.status != row.status || produced.body != nil || produced.headers.Values != nil || produced.retryAfter != "" {
									cancel()
									t.Fatalf("aggregate classifier changed producer facts: %+v", produced)
								}
							}
							cancel()
							if calls != 1 || body.closes != 1 || body.readBytes != 0 {
								t.Fatalf("producer/classifier effects = calls %d close %d read %d, want 1/1/0", calls, body.closes, body.readBytes)
							}
						}
					})
				}
			}
		})
	}
}

type handoffCauseBits uint8

const (
	handoffCanceled handoffCauseBits = 1 << iota
	handoffRequest
	handoffRedirect
	handoffContentType
	handoffBodyLimit
	handoffResponse
	handoffTransport
	handoffCauseDomain
)
const handoffTerminalBits = handoffCanceled | handoffRequest | handoffRedirect | handoffContentType | handoffBodyLimit

// Each bit names a compiler-visible identity the classifiers inspect. Exhaust
// every subset, not a few convenient joined pairs. Reverse and duplicate the
// same causes within each row; these are invariant checks, not extra quota rows.
func TestHTTPProducerClassifierRefusalLatticeLayerTriad(t *testing.T) {
	t.Parallel()
	identities := []struct {
		bit      handoffCauseBits
		identity error
	}{
		{handoffCanceled, core.ErrExchangeCancelled}, {handoffRequest, core.ErrExchangeRequest},
		{handoffRedirect, core.ErrExchangeRedirect}, {handoffContentType, core.ErrExchangeContentType},
		{handoffBodyLimit, core.ErrExchangeBodyLimit}, {handoffResponse, core.ErrExchangeResponse},
		{handoffTransport, core.ErrExchangeTransport},
	}
	for bits := range handoffCauseDomain {
		row := struct {
			bits      handoffCauseBits
			primary   handoffPrimary
			wantRetry bool
		}{bits: bits, primary: handoffRefusal, wantRetry: bits&handoffTerminalBits == 0}
		t.Run(fmt.Sprintf("cause_set_%d_primary_%d", row.bits, row.primary), func(t *testing.T) {
			t.Parallel()
			target, err := core.ParseHTTPEndpoint("https://provider.example.test/refusal-domain")
			if err != nil {
				t.Fatal(err)
			}
			limit, err := core.NewByteCount(2)
			if err != nil {
				t.Fatal(err)
			}
			causes := []error{io.ErrClosedPipe}
			for _, identity := range identities {
				if row.bits&identity.bit != 0 {
					causes = append(causes, identity.identity)
				}
			}
			permutations := [][]error{slices.Clone(causes), slices.Clone(causes), append(slices.Clone(causes), causes...)}
			slices.Reverse(permutations[1])
			for order, causes := range permutations {
				for _, stream := range []bool{false, true} {
					for _, canceled := range []bool{false, true} {
						calls := 0
						ctx, cancel := context.WithCancel(t.Context())
						defer cancel()
						native := errors.Join(causes...)
						goClient := &http.Client{Transport: replayHandoffTransport(func(*http.Request) (*http.Response, error) { calls++; return nil, native })}
						var producedCause error
						var class attemptDisposition
						var gotErr error
						if stream {
							client, err := NewClient(goClient)
							if err != nil {
								t.Fatal(err)
							}
							var destination bytes.Buffer
							produced, producerErr := Download(DownloadCall{Context: ctx, Client: client, Request: DownloadRequest{Target: target, Destination: &destination, ExpectedStatus: core.HTTPStatusOK(), ResponseBodyLimit: limit, Semantics: RequestSemantics{Method: MethodGet, Replay: ReplaySingleAttempt}}, Policy: StreamPolicy{OperationTimeout: runtimeAgreementPolicy(t).ReadTimeout, AttemptTimeout: runtimeAgreementPolicy(t).ReadTimeout, ErrorBodyLimit: limit, Redirect: RedirectPolicy{Mode: RedirectReject}}})
							if produced.Metadata.Status != (core.HTTPStatusCode{}) || produced.Metadata.Attempts != 0 || produced.Metadata.Bytes != (core.ByteLength{}) || produced.Metadata.Headers.Values != nil || produced.DeclaredRequestBytes != (core.ByteLength{}) || destination.Len() != 0 {
								t.Fatalf("refused stream producer emitted facts: %+v", produced)
							}
							producedCause = producerErr
							// Producer refusal identities are checked before the classifier below.
							for _, cause := range causes {
								if !errors.Is(producedCause, cause) {
									t.Fatalf("stream producer lost %v in %v", cause, producedCause)
								}
							}
							if canceled {
								cancel()
							}
							admitted, admissionErr := admitStreamAttemptResponse(produced, producerErr, 1)
							retry, decisionErr := replayStreamDecision(ctx, admissionErr)
							gotErr = decisionErr
							class = attemptComplete
							if retry {
								class = attemptRetry
							}
							if admitted.Metadata.Status != (core.HTTPStatusCode{}) || admitted.Metadata.Attempts != 0 || admitted.Metadata.Bytes != (core.ByteLength{}) || admitted.Metadata.Headers.Values != nil || admitted.DeclaredRequestBytes != (core.ByteLength{}) {
								t.Fatalf("classifier invented stream evidence: %+v", admitted)
							}
						} else {
							semantics := RequestSemantics{Method: MethodGet, Replay: ReplaySafe}
							produced, producerErr := executeAggregateAttempt(aggregateAttempt{context: ctx, client: goClient, request: aggregateRequest{target: target, semantics: semantics, expectedStatus: core.HTTPStatusOK()}, timeout: runtimeAgreementPolicy(t).ReadTimeout, limit: limit})
							if produced.status != (core.HTTPStatusCode{}) || produced.body != nil || produced.headers.Values != nil || produced.retryAfter != "" {
								t.Fatalf("refused aggregate producer emitted facts: %+v", produced)
							}
							producedCause = producerErr
							for _, cause := range causes {
								if !errors.Is(producedCause, cause) {
									t.Fatalf("aggregate producer lost %v in %v", cause, producedCause)
								}
							}
							if canceled {
								cancel()
							}
							class, gotErr = classifyAggregateAttempt(aggregateAttemptResult{operationContext: ctx, cause: producerErr, semantics: semantics, response: produced, expected: core.HTTPStatusOK(), attemptsRemaining: 1})
						}
						wantClass := attemptComplete
						if row.wantRetry && !canceled {
							wantClass = attemptRetry
						}
						if class != wantClass {
							t.Errorf("stream=%t order=%d refusal classification = %v/%v, want %v", stream, order, class, gotErr, wantClass)
						}
						if canceled && !errors.Is(gotErr, context.Canceled) {
							t.Errorf("classifier lost operation cancellation: %v", gotErr)
						}
						if stream || !row.wantRetry || canceled {
							for _, cause := range causes {
								if !errors.Is(gotErr, cause) {
									t.Errorf("stream=%t order=%d classifier lost %v in %v", stream, order, cause, gotErr)
								}
							}
						} else if gotErr != nil {
							t.Errorf("scheduled aggregate retry error = %v, want nil with original producer cause retained", gotErr)
						}
						if calls != 1 {
							t.Fatalf("classification performed %d provider effects, want one producer effect", calls)
						}

						cancel()
					}
				}
			}
		})
	}
}
