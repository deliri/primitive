package cloudflare

import (
	"bytes"
	"context"
	"errors"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

// ZoneID and AccountID cannot be exchanged even when their wire grammar agrees.
type ZoneID struct{ value string }

func ParseZoneID(value string) (ZoneID, error) {
	if !hexIdentity(value) {
		return ZoneID{}, core.ErrCloudflareBinding
	}
	return ZoneID{value: value}, nil
}
func (z ZoneID) Validate() error { _, err := ParseZoneID(z.value); return err }
func (z ZoneID) String() string  { return z.value }

// CachePurgeOperationID is optional provider metadata. Zero means the provider
// omitted its operation ID; it is never evidence that an object was absent.
type CachePurgeOperationID string

func (id CachePurgeOperationID) Validate() error {
	if !utf8.ValidString(string(id)) || utf8.RuneCountInString(string(id)) > core.CloudflareCachePurgeIDMaximumCharacters {
		return core.ErrCloudflareResponse
	}
	return nil
}

type CacheServerOptions struct {
	Zone           ZoneID
	Token          APIToken
	ResponseLimits core.StrictJSONLimits
}

func (o CacheServerOptions) Validate() error {
	if err := errors.Join(o.Zone.Validate(), o.Token.Validate(), o.ResponseLimits.Validate()); err != nil {
		return contractError(err)
	}
	return nil
}

// CacheServer owns only zone-bound cache API calls. Application policy chooses
// when to purge, and must observe delivery separately to establish absence.
type CacheServer struct {
	api  apiServer
	zone ZoneID
}

func NewCacheServer(client exchange.Client, options CacheServerOptions) (CacheServer, error) {
	if err := options.Validate(); err != nil {
		return CacheServer{}, err
	}
	root, err := core.ParseHTTPEndpoint("https://" + core.CloudflareAPIHost + core.CloudflareAPIZonesPath + options.Zone.value)
	if err != nil {
		return CacheServer{}, contractError(err)
	}
	api, err := openAPIServer(apiServer{client: client, token: options.Token, root: root, limits: options.ResponseLimits})
	if err != nil {
		return CacheServer{}, err
	}
	return CacheServer{api: api, zone: options.Zone}, nil
}
func (s CacheServer) Validate() error { return errors.Join(s.api.Validate(), s.zone.Validate()) }
func (s *CacheServer) Close() error {
	if s == nil {
		return core.ErrCloudflareContract
	}
	return s.api.token.Close()
}

// CacheFilePurgeRequest names exactly one complete cache key. Hosts, prefixes,
// wildcard purges and custom cache-key header dimensions are separate APIs.
type CacheFilePurgeRequest struct{ URL core.HTTPEndpoint }

func (r CacheFilePurgeRequest) Validate() error {
	if err := r.URL.Validate(); err != nil {
		return contractError(err)
	}
	return nil
}

// CachePurgeReceipt reports provider acceptance, not observed cache absence.
type CachePurgeReceipt struct {
	URL         core.HTTPEndpoint
	Zone        ZoneID
	OperationID CachePurgeOperationID
	Accepted    bool
}

func (r CachePurgeReceipt) Validate() error {
	if !r.Accepted {
		return core.ErrCloudflareResponse
	}
	return errors.Join(r.URL.Validate(), r.Zone.Validate(), r.OperationID.Validate())
}

type cacheFilePurgeWire struct {
	Files [1]string `json:"files"`
}

func (w cacheFilePurgeWire) Validate() error {
	u, err := core.ParseHTTPEndpoint(w.Files[0])
	if err != nil {
		return contractError(err)
	}
	return (CacheFilePurgeRequest{URL: u}).Validate()
}

type cachePurgeWire struct {
	ID CachePurgeOperationID `json:"id,omitempty"`
}

func (w cachePurgeWire) Validate() error { return w.ID.Validate() }

func (s CacheServer) PurgeFile(ctx context.Context, request CacheFilePurgeRequest, policy exchange.StreamPolicy) (CachePurgeReceipt, error) {
	if err := errors.Join(s.Validate(), request.Validate()); err != nil {
		return CachePurgeReceipt{}, err
	}
	body, err := core.MarshalCanonicalJSONDocument(cacheFilePurgeWire{Files: [1]string{request.URL.String()}})
	if err != nil {
		return CachePurgeReceipt{}, contractError(err)
	}
	result, err := executeAPI[cachePurgeWire](ctx, s.api, apiIntent{suffix: core.CloudflareCachePurgePath, source: bytes.NewReader(body), media: core.HTTPMediaTypeJSON(), method: exchange.MethodPost}, policy)
	if err != nil {
		return CachePurgeReceipt{}, err
	}
	receipt := CachePurgeReceipt{URL: request.URL, Zone: s.zone, OperationID: result.ID, Accepted: true}
	if err := receipt.Validate(); err != nil {
		return CachePurgeReceipt{}, responseError(err)
	}
	return receipt, nil
}

func (ZoneID) cloudflareCapabilityWrapper()           {}
func (CacheServer) cloudflareCapabilityWrapper()      {}
func (CacheServerOptions) cloudflareProtocolFact()    {}
func (CacheFilePurgeRequest) cloudflareProtocolFact() {}
func (CachePurgeReceipt) cloudflareProtocolFact()     {}
func (cacheFilePurgeWire) cloudflareProtocolFact()    {}
func (cachePurgeWire) cloudflareProtocolFact()        {}
