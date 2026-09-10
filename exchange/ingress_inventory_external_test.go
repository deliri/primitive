package exchange_test

import (
	"testing"

	"github.com/deliri/primitive/v2026/exchange"
)

func TestPublicIngressHasCompilerBoundSemanticFuzz(t *testing.T) {
	t.Parallel()
	declarations := exchange.ExchangeIngressDeclarationsForTest(t)
	coverage := append(exchange.InternalIngressCoverageForTest(), externalIngressCoverage()...)
	for _, declaration := range declarations {
		t.Run(declaration.Symbol, func(t *testing.T) {
			t.Parallel()
			var matches []exchange.IngressCoverageForTest
			for _, entry := range coverage {
				if exchange.IngressSymbolForTest(entry.Door) == declaration.Symbol {
					matches = append(matches, entry)
				}
			}
			if len(matches) != 1 {
				t.Fatalf("%s:%d public door has %d compiler-bound inventory entries, want exactly one", declaration.File, declaration.Line, len(matches))
			}
			entry := matches[0]
			switch entry.Kind {
			case exchange.IngressFuzzForTest:
				if entry.Fuzz == nil {
					t.Fatalf("ingress=%+v, want a bound semantic fuzz target", entry)
				}
			case exchange.IngressCapabilityForTest:
				if entry.Proof == nil || entry.Reason == "" {
					t.Fatalf("ingress=%+v, want a bound proof and nonempty reason", entry)
				}
			case exchange.IngressProjectionForTest:
				if entry.Reason == "" {
					t.Fatalf("ingress=%+v, want a nonempty ownership reason", entry)
				}
			default:
				t.Fatalf("public ingress remains unfinished: %s", entry.Reason)
			}
		})
	}
	for _, entry := range coverage {
		symbol := exchange.IngressSymbolForTest(entry.Door)
		matches := 0
		for _, declaration := range declarations {
			if declaration.Symbol == symbol {
				matches++
			}
		}
		if matches != 1 {
			t.Errorf("inventory binding %q resolves to %d production declarations, want one", symbol, matches)
		}
	}
}

func externalIngressCoverage() []exchange.IngressCoverageForTest {
	entries := []exchange.IngressCoverageForTest{
		{Door: exchange.SendTo, Kind: exchange.IngressFuzzForTest, Fuzz: FuzzResponseDestinationPreservesDelivery},
		{Door: exchange.SendNoBodyTo, Kind: exchange.IngressFuzzForTest, Fuzz: FuzzResponseDestinationPreservesDelivery},
		{Door: exchange.Client.OfficialSDKResponseTransport, Kind: exchange.IngressCapabilityForTest, Proof: TestClientOfficialSDKTransportLayerTriad, Reason: "Projects caller-owned Go transport through a previously validated SDK boundary."},
		{Door: (*exchange.BasicAuthorizationIdentity).UnmarshalJSON, Kind: exchange.IngressFuzzForTest, Fuzz: FuzzBasicAuthorizationIdentityJSONSemanticClosure},
		{Door: exchange.ReceiveBasicAuthorization, Kind: exchange.IngressFuzzForTest, Fuzz: FuzzReceiveBasicAuthorizationSemanticClosure},
		{Door: exchange.NewBearerAuthorizationHeader, Kind: exchange.IngressFuzzForTest, Fuzz: FuzzBearerAuthorizationTokenSemanticClosure},
		{Door: exchange.ReceiveBearerAuthorization, Kind: exchange.IngressFuzzForTest, Fuzz: FuzzReceiveBearerAuthorizationSemanticClosure},
		{Door: exchange.NewHeaderValue, Kind: exchange.IngressFuzzForTest, Fuzz: FuzzHeaderValueMatchesNetHTTPAndExchangeBounds},
		{Door: exchange.ResolveClientAddress, Kind: exchange.IngressFuzzForTest, Fuzz: FuzzClientAddressResolutionSemanticClosure},
		{Door: exchange.SendJSON[replayBoundDocument, transportDocument], Kind: exchange.IngressFuzzForTest, Fuzz: FuzzSendJSONReplayNoBodyAndSocketResponses},
		{Door: exchange.SendReplayBoundJSON[replayBoundDocument, transportDocument], Kind: exchange.IngressFuzzForTest, Fuzz: FuzzSendJSONReplayNoBodyAndSocketResponses},
		{Door: exchange.SendNoBodyJSON[transportDocument], Kind: exchange.IngressFuzzForTest, Fuzz: FuzzSendJSONReplayNoBodyAndSocketResponses},
		{Door: exchange.SendSocketJSON[replayBoundDocument, transportDocument], Kind: exchange.IngressFuzzForTest, Fuzz: FuzzSendJSONReplayNoBodyAndSocketResponses},
		{Door: exchange.SendReplayBoundSocketJSON[replayBoundDocument, transportDocument], Kind: exchange.IngressFuzzForTest, Fuzz: FuzzSendJSONReplayNoBodyAndSocketResponses},
		{Door: exchange.SendNoBodyBounded, Kind: exchange.IngressFuzzForTest, Fuzz: FuzzCapturedHeadersPreserveExactSelection},
		{Door: exchange.ReceiveJSON[replayBoundDocument, *replayBoundDocument], Kind: exchange.IngressFuzzForTest, Fuzz: FuzzReceiveJSONProjectedJSONAndSocketJSONCustody},
		{Door: exchange.ReceiveReplayBoundJSON[replayBoundDocument, *replayBoundDocument], Kind: exchange.IngressFuzzForTest, Fuzz: FuzzReceiveJSONProjectedJSONAndSocketJSONCustody},
		{Door: exchange.ReceiveProjectedJSON[replayBoundDocument, *replayBoundDocument], Kind: exchange.IngressFuzzForTest, Fuzz: FuzzReceiveJSONProjectedJSONAndSocketJSONCustody},
		{Door: exchange.ReceiveSocketJSON[replayBoundDocument, *replayBoundDocument], Kind: exchange.IngressFuzzForTest, Fuzz: FuzzReceiveJSONProjectedJSONAndSocketJSONCustody},
		{Door: exchange.ReceiveReplayBoundSocketJSON[replayBoundDocument, *replayBoundDocument], Kind: exchange.IngressFuzzForTest, Fuzz: FuzzReceiveJSONProjectedJSONAndSocketJSONCustody},
		{Door: exchange.ReceiveBounded, Kind: exchange.IngressFuzzForTest, Fuzz: FuzzReceiveBoundedAndNoBodyCustody},
		{Door: exchange.ReceiveNoBody, Kind: exchange.IngressFuzzForTest, Fuzz: FuzzReceiveBoundedAndNoBodyCustody},
		{Door: exchange.ReceiveStream, Kind: exchange.IngressFuzzForTest, Fuzz: FuzzReceiveStreamCustodySemanticClosure},
		{Door: exchange.Download, Kind: exchange.IngressFuzzForTest, Fuzz: FuzzReplayStreamPreservesDownloadAndRoundTripObservation},
		{Door: exchange.ReplayStream, Kind: exchange.IngressFuzzForTest, Fuzz: FuzzReplayStreamPreservesDownloadAndRoundTripObservation},
		{Door: exchange.SocketServerCall.UniqueHeader, Kind: exchange.IngressFuzzForTest, Fuzz: FuzzSocketHeaderQueryAndPathObservations},
		{Door: exchange.SocketServerCall.RawQuery, Kind: exchange.IngressFuzzForTest, Fuzz: FuzzSocketHeaderQueryAndPathObservations},
		{Door: exchange.SocketServerCall.MatchesEndpointPath, Kind: exchange.IngressFuzzForTest, Fuzz: FuzzSocketHeaderQueryAndPathObservations},
		{Door: exchange.ValidateSocketCallPath, Kind: exchange.IngressFuzzForTest, Fuzz: FuzzSocketHeaderQueryAndPathObservations},
		{Door: exchange.SocketServerCall.VerifiedClientCertificateDigest, Kind: exchange.IngressFuzzForTest, Fuzz: FuzzSocketVerifiedClientCertificateIdentity},
		{Door: (*exchange.OfficialSDKResponseRepresentation).UnmarshalJSON, Kind: exchange.IngressFuzzForTest, Fuzz: FuzzOfficialSDKResponseRepresentationJSONSemanticClosure},
		{Door: exchange.NewOfficialSDKResponseTransport, Kind: exchange.IngressFuzzForTest, Fuzz: FuzzOfficialSDKResponseTransportSemanticBoundary},
		{Door: exchange.NewOfficialSDKHTTPClient, Kind: exchange.IngressFuzzForTest, Fuzz: FuzzOfficialSDKResponseTransportSemanticBoundary},
		{Door: exchange.NewClient, Kind: exchange.IngressCapabilityForTest, Proof: TestClientTimeoutOwnershipLayerTriad, Reason: "Admits a Go client capability, not an external representation."},
		{Door: exchange.NewStandardClient, Kind: exchange.IngressCapabilityForTest, Proof: TestNewStandardClientProducesTheShapeNewClientDemands, Reason: "Creates the standard Go client without input material."},
		{Door: exchange.NewClientSocket, Kind: exchange.IngressFuzzForTest, Fuzz: FuzzSendJSONReplayNoBodyAndSocketResponses},
		{Door: exchange.NewServerSocket, Kind: exchange.IngressFuzzForTest, Fuzz: FuzzReceiveJSONProjectedJSONAndSocketJSONCustody},
		{Door: exchange.NewSocketServerCall, Kind: exchange.IngressCapabilityForTest, Proof: TestSocketServerCallLayerTriad, Reason: "Seals existing Go request/writer capabilities; all material observations have separate fuzz entries."},
		{Door: exchange.SocketServerCall.Context, Kind: exchange.IngressCapabilityForTest, Proof: TestSocketServerCallLayerTriad, Reason: "Projects the sealed Go request context without decoding material."},
		{Door: exchange.SocketServerCall.WithContext, Kind: exchange.IngressCapabilityForTest, Proof: TestSocketContextDerivationLayerTriad, Reason: "Derives a Go request with the supplied context capability."},
		{Door: exchange.SocketServerCall.ServeHTTP, Kind: exchange.IngressCapabilityForTest, Proof: TestSocketServerCallLayerTriad, Reason: "Passes the exact Go request and writer to a caller-owned Go handler."},
	}
	for _, door := range []any{(*exchange.ServerRuntime).Ready, (*exchange.ServerRuntime).Address, (*exchange.ServerRuntime).Serve, (*exchange.ServerRuntime).ServeListener, (*exchange.ServerRuntime).Close, (*exchange.ServerRuntime).Shutdown} {
		entries = append(entries, exchange.IngressCoverageForTest{Door: door, Kind: exchange.IngressCapabilityForTest, Proof: TestServerRuntimeLayerTriad, Reason: "Owns a Go server/listener lifecycle using typed configuration; no independent external document decoding."})
	}
	for _, door := range []any{exchange.BearerAuthorizationMatches, exchange.HTTPStatusAccepted, exchange.ClientAddressAuthority.OffWireEnum, exchange.TrustedProxyPrefixes.Count, exchange.ReplayMode.OffWireEnum, exchange.RedirectMode.OffWireEnum, exchange.StandardMediaType.OffWireEnum, exchange.StandardHeader.OffWireEnum, exchange.RequestSemantics.AllowsRetry, exchange.StatusError.Status, exchange.StatusError.Expected, exchange.StatusError.Error, exchange.StatusError.Unwrap, exchange.RetryExhaustedError.Attempts, exchange.RetryExhaustedError.Cause, exchange.RetryExhaustedError.Error, exchange.RetryExhaustedError.Unwrap, exchange.StandardHeader.Name, exchange.StandardMediaType.HTTPMediaType} {
		entries = append(entries, exchange.IngressCoverageForTest{Door: door, Kind: exchange.IngressProjectionForTest, Reason: "Pure projection/comparison of already typed facts; its external representation is admitted by the owning door."})
	}
	entries = append(entries, exchange.IngressCoverageForTest{Door: exchange.RoundTripStream, Kind: exchange.IngressFuzzForTest, Fuzz: FuzzReplayStreamPreservesDownloadAndRoundTripObservation})

	return entries
}
