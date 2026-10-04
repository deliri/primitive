package cloudflare

import (
	"encoding/hex"
	"errors"
	"net/url"
	"strconv"
	"strings"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

// R2GrantInput carries a grant across the server/client wall. Account and
// jurisdiction are the caller's expected authority, not inferred from a URL.
// Parsing proves structure and binding; R2 independently verifies the signature.
type R2GrantInput struct {
	Account      AccountID
	ContentType  core.HTTPMediaType
	Endpoint     core.HTTPEndpoint
	Jurisdiction R2Jurisdiction
	Method       exchange.Method
}

func (r R2GrantInput) Validate() error {
	if err := errors.Join(r.Endpoint.Validate(), r.Account.Validate(), r.Jurisdiction.Validate(), validateR2Method(r.Method)); err != nil {
		return contractError(err)
	}
	label, err := r.Jurisdiction.label()
	if err != nil {
		return err
	}
	u := r.Endpoint.HTTPURL()
	if u.Scheme != core.SchemeHTTPS || u.Host != r.Account.value+label+core.CloudflareR2HostSuffix {
		return core.ErrCloudflareBinding
	}
	if err := validateR2ObjectPath(u.Path); err != nil {
		return err
	}
	return validateR2Query(u.RawQuery, r.Method, r.ContentType)
}
func ParseR2Grant(input R2GrantInput) (R2Grant, error) {
	if err := input.Validate(); err != nil {
		return R2Grant{}, err
	}
	return R2Grant{endpoint: input.Endpoint, method: input.Method, contentType: input.ContentType}, nil
}

func validateR2ObjectPath(path string) error {
	path, ok := strings.CutPrefix(path, "/")
	if !ok {
		return core.ErrCloudflareBinding
	}
	bucket, key, ok := strings.Cut(path, "/")
	if !ok {
		return core.ErrCloudflareBinding
	}
	_, bucketErr := ParseR2Bucket(bucket)
	_, keyErr := ParseR2Key(key)
	return errors.Join(bucketErr, keyErr)
}

func validateR2Query(raw string, method exchange.Method, media core.HTTPMediaType) error {
	query, err := parseR2Query(raw)
	if err != nil {
		return err
	}
	if err := validateR2SignatureFields(query); err != nil {
		return err
	}
	wantHeaders := "host"
	if !media.IsZero() {
		if method != exchange.MethodPut || media.Validate() != nil {
			return core.ErrCloudflareBinding
		}
		wantHeaders = "content-type;host"
	}
	if query.Get(core.CloudflareR2QuerySignedHeaders) != wantHeaders {
		return core.ErrCloudflareBinding
	}
	return nil
}

func parseR2Query(raw string) (url.Values, error) {
	if len(raw) > core.CloudflareR2QueryMaximumBytes {
		return nil, core.ErrCloudflareBinding
	}
	query, err := url.ParseQuery(raw)
	if err != nil {
		return nil, contractError(err)
	}
	if len(query) != 6 {
		return nil, core.ErrCloudflareBinding
	}
	for _, key := range [...]string{core.CloudflareR2QueryAlgorithm, core.CloudflareR2QuerySigningIdentity, core.CloudflareR2QueryDate, core.CloudflareR2QueryExpires, core.CloudflareR2QuerySignature, core.CloudflareR2QuerySignedHeaders} {
		if len(query[key]) != 1 || query.Get(key) == "" {
			return nil, core.ErrCloudflareBinding
		}
	}
	if query.Get(core.CloudflareR2QueryAlgorithm) != core.CloudflareR2Algorithm {
		return nil, core.ErrCloudflareBinding
	}
	return query, nil
}

func validateR2SignatureFields(query url.Values) error {
	if err := validateR2Credential(query); err != nil {
		return err
	}
	if err := validateR2Expiry(query.Get(core.CloudflareR2QueryExpires)); err != nil {
		return err
	}
	signature := query.Get(core.CloudflareR2QuerySignature)
	if len(signature) != core.CloudflareR2SignatureHexBytes {
		return core.ErrCloudflareBinding
	}
	var decoded [core.CloudflareR2SignatureHexBytes / 2]byte
	if _, err := hex.Decode(decoded[:], []byte(signature)); err != nil {
		return contractError(err)
	}
	return nil
}

func validateR2Credential(query url.Values) error {
	stamp, err := temporal.ParseCompactUTC(query.Get(core.CloudflareR2QueryDate))
	if err != nil {
		return contractError(err)
	}
	canonical, err := stamp.CompactUTC()
	if err != nil {
		return err
	}
	credential := strings.Split(query.Get(core.CloudflareR2QuerySigningIdentity), "/")
	if len(credential) != 5 || !visibleSecret([]byte(credential[0])) || credential[1] != canonical[:8] || credential[2] != core.CloudflareR2SigningRegion || credential[3] != core.CloudflareR2SigningService || credential[4] != core.CloudflareR2CredentialTerminator {
		return core.ErrCloudflareBinding
	}
	return nil
}

func validateR2Expiry(value string) error {
	expires, err := strconv.ParseUint(value, 10, 32)
	if err != nil || expires < 1 || expires > core.CloudflareR2PresignMaximumSeconds {
		return core.ErrCloudflareBinding
	}
	return nil
}
