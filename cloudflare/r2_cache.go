package cloudflare

import (
	"strconv"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

// R2CacheControl carries an optional, exact max-age for object metadata.
// Zero omits the header; NewR2CacheControl with zero duration emits max-age=0.
// The caller owns the caching decision. R2 signs and stores the chosen value.
type R2CacheControl struct {
	maxAge  temporal.Duration
	present bool
}

func NewR2CacheControl(maxAge temporal.Duration) (R2CacheControl, error) {
	value := R2CacheControl{maxAge: maxAge, present: true}
	if err := value.Validate(); err != nil {
		return R2CacheControl{}, err
	}
	return value, nil
}

func (c R2CacheControl) Validate() error {
	if !c.present && !c.maxAge.IsZero() {
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
	if c.present && !write {
		return core.ErrCloudflareBinding
	}
	return nil
}

func (c R2CacheControl) headers() exchange.Headers {
	if !c.present {
		return exchange.Headers{}
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
