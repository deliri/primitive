package cloudflare

import (
	"bytes"
	"context"
	"errors"
	"strings"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

// CachePurgePrefix selects Cloudflare's native host/path prefix invalidation,
// including query and header cache variants. It may invalidate longer paths
// sharing the prefix. The caller owns that scope; this type does not list,
// track, or interpret objects. Query-bearing input is refused, never widened.
type CachePurgePrefix struct{ endpoint core.HTTPEndpoint }

func NewCachePurgePrefix(endpoint core.HTTPEndpoint) (CachePurgePrefix, error) {
	p := CachePurgePrefix{endpoint: endpoint}
	if err := p.Validate(); err != nil {
		return CachePurgePrefix{}, err
	}
	return p, nil
}

func (p CachePurgePrefix) Validate() error {
	if err := p.endpoint.Validate(); err != nil {
		return errors.Join(core.ErrCloudflareBinding, err)
	}
	u := p.endpoint.HTTPURL()
	if u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || strings.Count(u.Path, "/") > core.CloudflareCachePrefixMaximumSeparators {
		return core.ErrCloudflareBinding
	}
	return nil
}

func (p CachePurgePrefix) String() string {
	if p.Validate() != nil {
		return ""
	}
	u := p.endpoint.HTTPURL()
	return u.Host + u.EscapedPath()
}

type CachePrefixPurgeRequest struct{ Prefix CachePurgePrefix }

func (r CachePrefixPurgeRequest) Validate() error {
	if err := r.Prefix.Validate(); err != nil {
		return contractError(err)
	}
	return nil
}

// CachePrefixPurgeReceipt is provider acceptance for exactly this prefix/zone.
// It does not prove public absence or deletion from the origin.
type CachePrefixPurgeReceipt struct {
	Prefix      CachePurgePrefix
	Zone        ZoneID
	OperationID CachePurgeOperationID
	Accepted    bool
}

func (r CachePrefixPurgeReceipt) Validate() error {
	if !r.Accepted {
		return core.ErrCloudflareResponse
	}
	return errors.Join(r.Prefix.Validate(), r.Zone.Validate(), r.OperationID.Validate())
}

type cachePrefixPurgeWire struct {
	Prefixes [1]string `json:"prefixes"`
}

func (s CacheServer) PurgePrefix(ctx context.Context, request CachePrefixPurgeRequest, policy exchange.StreamPolicy) (CachePrefixPurgeReceipt, error) {
	if err := errors.Join(s.Validate(), request.Validate()); err != nil {
		return CachePrefixPurgeReceipt{}, err
	}
	body, err := core.MarshalCanonicalJSONDocument(cachePrefixPurgeWire{Prefixes: [1]string{request.Prefix.String()}})
	if err != nil {
		return CachePrefixPurgeReceipt{}, contractError(err)
	}
	result, err := executeAPI[cachePurgeWire](ctx, s.api, apiIntent{suffix: core.CloudflareCachePurgePath, source: bytes.NewReader(body), media: core.HTTPMediaTypeJSON(), method: exchange.MethodPost}, policy)
	if err != nil {
		return CachePrefixPurgeReceipt{}, err
	}
	receipt := CachePrefixPurgeReceipt{Prefix: request.Prefix, Zone: s.zone, OperationID: result.ID, Accepted: true}
	if err := receipt.Validate(); err != nil {
		return CachePrefixPurgeReceipt{}, responseError(err)
	}
	return receipt, nil
}

func (CachePurgePrefix) cloudflareCapabilityWrapper()   {}
func (CachePrefixPurgeRequest) cloudflareProtocolFact() {}
func (CachePrefixPurgeReceipt) cloudflareProtocolFact() {}
func (cachePrefixPurgeWire) cloudflareProtocolFact()    {}
