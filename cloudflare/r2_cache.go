package cloudflare

import (
	"strconv"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

// R2CacheControl carries the exact optional caching directive for object metadata.
// Zero omits the header; NewR2CacheControl with zero duration emits max-age=0.
// The caller owns the caching decision. R2 signs and stores the chosen value.
type r2CacheMode uint8

const (
	r2CacheOmit r2CacheMode = iota
	r2CacheMaxAge
	r2CacheNoStore
)

type R2CacheControl struct {
	maxAge temporal.Duration
	mode   r2CacheMode
}

func NewR2CacheControl(maxAge temporal.Duration) (R2CacheControl, error) {
	value := R2CacheControl{maxAge: maxAge, mode: r2CacheMaxAge}
	if err := value.Validate(); err != nil {
		return R2CacheControl{}, err
	}
	return value, nil
}

// R2NoStoreCacheControl forbids retaining an original beyond its authorized read.
func R2NoStoreCacheControl() R2CacheControl { return R2CacheControl{mode: r2CacheNoStore} }

func (c R2CacheControl) Validate() error {
	switch c.mode {
	case r2CacheOmit, r2CacheNoStore:
		if !c.maxAge.IsZero() {
			return core.ErrCloudflareBinding
		}
	case r2CacheMaxAge:
	default:
		return core.ErrCloudflareBinding
	}
	if c.maxAge.Validate() != nil || c.maxAge.Nanoseconds()%int64(temporal.NanosecondsPerSecond) != 0 || c.maxAge.Nanoseconds()/int64(temporal.NanosecondsPerSecond) > core.CloudflareR2CacheMaxAgeMaximumSeconds {
		return core.ErrCloudflareBinding
	}
	return nil
}

func (c R2CacheControl) validateWrite(write bool) error {
	if err := c.Validate(); err != nil {
		return err
	}
	if c.mode != r2CacheOmit && !write {
		return core.ErrCloudflareBinding
	}
	return nil
}

func (c R2CacheControl) headers() exchange.Headers {
	if c.mode == r2CacheOmit {
		return exchange.Headers{}
	}
	if c.mode == r2CacheNoStore {
		return exchange.Headers{Values: []exchange.Header{cloudflareRequestHeader(exchange.StandardHeaderCacheControl.String(), core.CloudflareR2CacheNoStore)}}
	}
	value := core.CloudflareR2CacheMaxAgePrefix + strconv.FormatInt(c.maxAge.Nanoseconds()/int64(temporal.NanosecondsPerSecond), 10)
	return exchange.Headers{Values: []exchange.Header{cloudflareRequestHeader(exchange.StandardHeaderCacheControl.String(), value)}}
}

func r2WriteHeaders(conditions R2WriteConditions, cache R2CacheControl) exchange.Headers {
	headers := cache.headers()
	headers.Values = append(headers.Values, conditions.headers().Values...)
	return headers
}

func (R2CacheControl) cloudflareProtocolFact() {}
