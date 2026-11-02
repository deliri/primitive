package cloudflare

import (
	"bytes"
	"context"
	"encoding/json/jsontext"
	"errors"
	"math"
	"strings"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

// One fixed GraphQL Analytics query: one zone, one host, eyeball traffic, one
// aggregate group without dimensions. There is no query builder.
// https://developers.cloudflare.com/analytics/graphql-api/
// https://developers.cloudflare.com/analytics/graphql-api/sampling/
const (
	zoneAnalyticsGraphQLPath     = "/client/v4/graphql"
	zoneAnalyticsRequestEyeball  = "eyeball"
	zoneAnalyticsHTTPTrafficText = "query ZoneHTTPTraffic($zoneTag: string, $filter: filter) { viewer { zones(filter: {zoneTag: $zoneTag}) { zoneTag httpRequestsAdaptiveGroups(limit: 1, filter: $filter) { count sum { visits edgeResponseBytes } avg { sampleInterval } } } } }"
)

// DNS hostname grammar bounds (RFC 1035 section 2.3.4; RFC 3696 section 2).
const (
	zoneHostMaximumBytes      = 253
	zoneHostLabelMaximumBytes = 63
)

// ZoneHost is one exact lowercase DNS hostname matched against the request
// Host header. It is not normalized: uppercase, trailing dots, ports,
// wildcards and IP literals are refused rather than rewritten.
type ZoneHost struct{ value string }

func ParseZoneHost(value string) (ZoneHost, error) {
	if len(value) == 0 || len(value) > zoneHostMaximumBytes {
		return ZoneHost{}, core.ErrCloudflareContract
	}
	labels := strings.Split(value, ".")
	if len(labels) < 2 {
		return ZoneHost{}, core.ErrCloudflareContract
	}
	for _, label := range labels {
		if !zoneHostLabel(label) {
			return ZoneHost{}, core.ErrCloudflareContract
		}
	}
	if strings.Trim(labels[len(labels)-1], "0123456789") == "" {
		return ZoneHost{}, core.ErrCloudflareContract
	}
	return ZoneHost{value: value}, nil
}
func (h ZoneHost) Validate() error { _, err := ParseZoneHost(h.value); return err }
func (h ZoneHost) String() string  { return h.value }

func zoneHostLabel(label string) bool {
	if len(label) == 0 || len(label) > zoneHostLabelMaximumBytes || label[0] == '-' || label[len(label)-1] == '-' {
		return false
	}
	for _, r := range label {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-') {
			return false
		}
	}
	return true
}

type ZoneAnalyticsServerOptions struct {
	Zone           ZoneID
	Token          APIToken
	ResponseLimits core.StrictJSONLimits
}

func (o ZoneAnalyticsServerOptions) Validate() error {
	if err := errors.Join(o.Zone.Validate(), o.Token.Validate(), o.ResponseLimits.Validate()); err != nil {
		return contractError(err)
	}
	return nil
}

// ZoneAnalyticsServer owns one zone-bound, read-only GraphQL Analytics query.
// The token needs only the zone Analytics Read permission.
type ZoneAnalyticsServer struct {
	zone ZoneID
	api  apiServer
}

func NewZoneAnalyticsServer(client exchange.Client, options ZoneAnalyticsServerOptions) (ZoneAnalyticsServer, error) {
	if err := options.Validate(); err != nil {
		return ZoneAnalyticsServer{}, err
	}
	root, err := core.ParseHTTPEndpoint("https://" + core.CloudflareAPIHost + zoneAnalyticsGraphQLPath)
	if err != nil {
		return ZoneAnalyticsServer{}, contractError(err)
	}
	api, err := openAPIServer(apiServer{client: client, token: options.Token, root: root, limits: options.ResponseLimits})
	if err != nil {
		return ZoneAnalyticsServer{}, err
	}
	return ZoneAnalyticsServer{api: api, zone: options.Zone}, nil
}
func (s ZoneAnalyticsServer) Validate() error {
	return errors.Join(s.api.Validate(), s.zone.Validate())
}
func (s *ZoneAnalyticsServer) Close() error {
	if s == nil {
		return core.ErrCloudflareContract
	}
	return s.api.token.Close()
}

// ZoneHTTPTrafficRequest selects one host over one half-open wall window
// [start, end). Both bounds are whole UTC seconds and the window is nonempty.
type ZoneHTTPTrafficRequest struct {
	Host   ZoneHost
	Window temporal.Interval
}

func (r ZoneHTTPTrafficRequest) Validate() error {
	_, err := r.filter()
	return err
}

func (r ZoneHTTPTrafficRequest) filter() (zoneAnalyticsFilterWire, error) {
	if err := r.Host.Validate(); err != nil {
		return zoneAnalyticsFilterWire{}, err
	}
	bounds, err := r.Window.Bounds()
	if err != nil {
		return zoneAnalyticsFilterWire{}, contractError(err)
	}
	order, err := bounds.Start.Compare(bounds.End)
	if err != nil || order != core.ComparisonLess {
		return zoneAnalyticsFilterWire{}, errors.Join(core.ErrCloudflareContract, err)
	}
	start, startErr := bounds.Start.RFC3339()
	end, endErr := bounds.End.RFC3339()
	if err := errors.Join(startErr, endErr); err != nil {
		return zoneAnalyticsFilterWire{}, contractError(err)
	}
	return zoneAnalyticsFilterWire{DatetimeGEQ: start, DatetimeLT: end, Host: r.Host.value, RequestSource: zoneAnalyticsRequestEyeball}, nil
}

// ZoneAnalyticsOutcome is closed. NoGroups means the provider returned no
// aggregate row for the window; it is not evidence of zero traffic.
type ZoneAnalyticsOutcome uint8

const (
	ZoneAnalyticsOutcomeUnknown ZoneAnalyticsOutcome = iota
	ZoneAnalyticsObserved
	ZoneAnalyticsNoGroups
	zoneAnalyticsOutcomeLimit
)

func zoneAnalyticsOutcomeNames() [zoneAnalyticsOutcomeLimit]string {
	return [...]string{"", "observed", "no_groups"}
}
func (outcome ZoneAnalyticsOutcome) Validate() error {
	if outcome <= ZoneAnalyticsOutcomeUnknown || outcome >= zoneAnalyticsOutcomeLimit || zoneAnalyticsOutcomeNames()[outcome] == "" {
		return core.ErrCloudflareResponse
	}
	return nil
}
func (outcome ZoneAnalyticsOutcome) IsValid() bool { return outcome.Validate() == nil }

// String is diagnostic text, not a new provider wire protocol.
func (outcome ZoneAnalyticsOutcome) String() string {
	if !outcome.IsValid() {
		return ""
	}
	return zoneAnalyticsOutcomeNames()[outcome]
}

// OffWireEnum declares this derived in-process outcome without generic codecs.
func (ZoneAnalyticsOutcome) OffWireEnum() {}

// ZoneAnalyticsSampleInterval is the provider's average sampling interval:
// 1 means unsampled; above 1 means counts and sums are sample estimates.
// Cloudflare defines the interval as inverse inclusion probability; averaging
// those intervals preserves the lower bound of one (a semantic inference).
// https://blog.cloudflare.com/how-we-make-sense-of-too-much-data/
type ZoneAnalyticsSampleInterval float64

func (i ZoneAnalyticsSampleInterval) Validate() error {
	value := float64(i)
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 1 {
		return core.ErrCloudflareResponse
	}
	return nil
}

// ZoneHTTPTraffic is one complete provider aggregate bound to the requested
// zone, host and window. Requests, Visits and EdgeResponseBytes are the
// provider's adaptive (possibly sampled) values, exposed with SampleInterval.
type ZoneHTTPTraffic struct {
	Zone              ZoneID
	Host              ZoneHost
	Window            temporal.Interval
	Requests          uint64
	Visits            uint64
	EdgeResponseBytes uint64
	SampleInterval    ZoneAnalyticsSampleInterval
	Outcome           ZoneAnalyticsOutcome
}

func (t ZoneHTTPTraffic) Validate() error {
	if err := errors.Join(t.Zone.Validate(), (ZoneHTTPTrafficRequest{Host: t.Host, Window: t.Window}).Validate()); err != nil {
		return err
	}
	switch t.Outcome {
	case ZoneAnalyticsObserved:
		return t.SampleInterval.Validate()
	case ZoneAnalyticsNoGroups:
		if t.Requests != 0 || t.Visits != 0 || t.EdgeResponseBytes != 0 || t.SampleInterval != 0 {
			return core.ErrCloudflareResponse
		}
		return nil
	default:
		return core.ErrCloudflareResponse
	}
}

// GraphQLIssue retains the provider's diagnostic message only.
type GraphQLIssue struct{ Message string }

// GraphQLRefusal reports a GraphQL document carrying errors. Any data in the
// same document is partial and discarded.
type GraphQLRefusal struct{ Issues []GraphQLIssue }

func (e GraphQLRefusal) Error() string { return core.ErrCloudflareResponse.Error() }
func (e GraphQLRefusal) Unwrap() error { return core.ErrCloudflareResponse }

type zoneAnalyticsQueryWire struct {
	Query     string                     `json:"query"`
	Variables zoneAnalyticsVariablesWire `json:"variables"`
}
type zoneAnalyticsVariablesWire struct {
	ZoneTag string                  `json:"zoneTag"`
	Filter  zoneAnalyticsFilterWire `json:"filter"`
}
type zoneAnalyticsFilterWire struct {
	DatetimeGEQ   string `json:"datetime_geq"`
	DatetimeLT    string `json:"datetime_lt"`
	Host          string `json:"clientRequestHTTPHost"`
	RequestSource string `json:"requestSource"`
}

// GraphQL error members other than message are opaque validated JSON at this
// provider wire boundary. Native jsontext.Value retains heterogeneous path,
// locations and future extension values. Core's strict grammar and json/v2 own
// decoding; these values never become application policy or retained traffic.
type graphQLIssueWire struct {
	Message    string         `json:"message"`
	Path       jsontext.Value `json:"path,omitzero"`
	Locations  jsontext.Value `json:"locations,omitzero"`
	Extensions jsontext.Value `json:"extensions,omitzero"`
}
type zoneAnalyticsEnvelope struct {
	Data   *zoneAnalyticsDataWire `json:"data"`
	Errors []graphQLIssueWire     `json:"errors"`
}
type zoneAnalyticsDataWire struct {
	Viewer *zoneAnalyticsViewerWire `json:"viewer"`
}
type zoneAnalyticsViewerWire struct {
	Zones []zoneAnalyticsZoneWire `json:"zones"`
}
type zoneAnalyticsZoneWire struct {
	ZoneTag string                   `json:"zoneTag"`
	Groups  []zoneAnalyticsGroupWire `json:"httpRequestsAdaptiveGroups"`
}
type zoneAnalyticsGroupWire struct {
	Count *uint64               `json:"count"`
	Sum   *zoneAnalyticsSumWire `json:"sum"`
	Avg   *zoneAnalyticsAvgWire `json:"avg"`
}
type zoneAnalyticsSumWire struct {
	Visits            *uint64 `json:"visits"`
	EdgeResponseBytes *uint64 `json:"edgeResponseBytes"`
}
type zoneAnalyticsAvgWire struct {
	SampleInterval *float64 `json:"sampleInterval"`
}

// ReadHTTP performs one POST of the fixed query. Refused input never reaches
// the provider. Errors, partial data, a foreign zone or an ambiguous shape
// yield the zero value; an empty group list yields ZoneAnalyticsNoGroups.
func (s ZoneAnalyticsServer) ReadHTTP(ctx context.Context, request ZoneHTTPTrafficRequest, policy exchange.StreamPolicy) (ZoneHTTPTraffic, error) {
	if err := errors.Join(s.Validate(), validatePolicy(policy)); err != nil {
		return ZoneHTTPTraffic{}, err
	}
	filter, err := request.filter()
	if err != nil {
		return ZoneHTTPTraffic{}, err
	}
	body, err := core.MarshalCanonicalJSONDocument(zoneAnalyticsQueryWire{Query: zoneAnalyticsHTTPTrafficText, Variables: zoneAnalyticsVariablesWire{ZoneTag: s.zone.value, Filter: filter}})
	if err != nil {
		return ZoneHTTPTraffic{}, contractError(err)
	}
	maximum, err := s.api.limits.DocumentMaximumBytes.Uint64()
	if err != nil {
		return ZoneHTTPTraffic{}, contractError(err)
	}
	response := boundedResponse{maximum: maximum}
	transferErr := s.api.transfer(ctx, apiIntent{source: bytes.NewReader(body), media: core.HTTPMediaTypeJSON(), method: exchange.MethodPost}, &response, policy)
	if transferErr != nil {
		// Only a completed refusal body may contribute provider diagnostics.
		if _, ok := errors.AsType[exchange.StatusError](transferErr); !ok || errors.Is(transferErr, core.ErrExchangeResponse) {
			return ZoneHTTPTraffic{}, transferErr
		}
	}
	envelope, err := core.DecodeStrictJSONStructure[zoneAnalyticsEnvelope](response.buffer.Bytes(), s.api.limits)
	if err != nil {
		return ZoneHTTPTraffic{}, errors.Join(transferErr, responseError(err))
	}
	if len(envelope.Errors) != 0 {
		refusal := GraphQLRefusal{Issues: make([]GraphQLIssue, 0, len(envelope.Errors))}
		for _, issue := range envelope.Errors {
			refusal.Issues = append(refusal.Issues, GraphQLIssue{Message: issue.Message})
		}
		return ZoneHTTPTraffic{}, errors.Join(transferErr, refusal)
	}
	if transferErr != nil {
		return ZoneHTTPTraffic{}, errors.Join(transferErr, core.ErrCloudflareResponse)
	}
	return envelope.traffic(s.zone, request)
}

func (e zoneAnalyticsEnvelope) traffic(zone ZoneID, request ZoneHTTPTrafficRequest) (ZoneHTTPTraffic, error) {
	if e.Data == nil || e.Data.Viewer == nil || len(e.Data.Viewer.Zones) != 1 {
		return ZoneHTTPTraffic{}, core.ErrCloudflareResponse
	}
	observed := e.Data.Viewer.Zones[0]
	if observed.ZoneTag != zone.value {
		return ZoneHTTPTraffic{}, core.ErrCloudflareBinding
	}
	result := ZoneHTTPTraffic{Zone: zone, Host: request.Host, Window: request.Window, Outcome: ZoneAnalyticsNoGroups}
	switch {
	case observed.Groups == nil || len(observed.Groups) > 1:
		return ZoneHTTPTraffic{}, core.ErrCloudflareResponse
	case len(observed.Groups) == 1:
		group := observed.Groups[0]
		if group.Count == nil || group.Sum == nil || group.Avg == nil || group.Sum.Visits == nil || group.Sum.EdgeResponseBytes == nil || group.Avg.SampleInterval == nil {
			return ZoneHTTPTraffic{}, core.ErrCloudflareResponse
		}
		result.Outcome = ZoneAnalyticsObserved
		result.Requests, result.Visits, result.EdgeResponseBytes = *group.Count, *group.Sum.Visits, *group.Sum.EdgeResponseBytes
		result.SampleInterval = ZoneAnalyticsSampleInterval(*group.Avg.SampleInterval)
	}
	if err := result.Validate(); err != nil {
		return ZoneHTTPTraffic{}, responseError(err)
	}
	return result, nil
}

func (ZoneHost) cloudflareCapabilityWrapper()              {}
func (ZoneAnalyticsServer) cloudflareCapabilityWrapper()   {}
func (ZoneAnalyticsServerOptions) cloudflareProtocolFact() {}
func (ZoneHTTPTrafficRequest) cloudflareProtocolFact()     {}
func (ZoneHTTPTraffic) cloudflareProtocolFact()            {}
func (GraphQLIssue) cloudflareProtocolFact()               {}
func (GraphQLRefusal) cloudflareProtocolFact()             {}
func (zoneAnalyticsQueryWire) cloudflareProtocolFact()     {}
func (zoneAnalyticsVariablesWire) cloudflareProtocolFact() {}
func (zoneAnalyticsFilterWire) cloudflareProtocolFact()    {}
func (graphQLIssueWire) cloudflareProtocolFact()           {}
func (zoneAnalyticsEnvelope) cloudflareProtocolFact()      {}
func (zoneAnalyticsDataWire) cloudflareProtocolFact()      {}
func (zoneAnalyticsViewerWire) cloudflareProtocolFact()    {}
func (zoneAnalyticsZoneWire) cloudflareProtocolFact()      {}
func (zoneAnalyticsGroupWire) cloudflareProtocolFact()     {}
func (zoneAnalyticsSumWire) cloudflareProtocolFact()       {}
func (zoneAnalyticsAvgWire) cloudflareProtocolFact()       {}
