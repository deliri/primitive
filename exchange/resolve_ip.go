package exchange

import (
	"context"
	"errors"
	"net"
	"net/netip"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

const (
	resolveHostMaximumBytes  = 253
	resolveLabelMaximumBytes = 63
	// ResolveIPMaximumAddresses bounds one lookup result. A larger answer is
	// refused rather than clipped, so a caller's every-address policy never
	// decides on a partial set.
	ResolveIPMaximumAddresses = 16
)

// ResolveIPRequest names one DNS host. Host is a letter-digit-hyphen name of
// at most 253 bytes with 1..63 byte labels, no trailing dot and a final label
// that is not all digits; IP literals, ports, schemes and paths are refused.
type ResolveIPRequest struct {
	Host string
}

func (r ResolveIPRequest) Validate() error {
	host := r.Host
	if host == "" || len(host) > resolveHostMaximumBytes {
		return core.ErrExchangeContract
	}
	labelStart, numericLabel := 0, true
	for index := 0; index <= len(host); index++ {
		if index == len(host) || host[index] == '.' {
			label := host[labelStart:index]
			if label == "" || len(label) > resolveLabelMaximumBytes || label[0] == '-' || label[len(label)-1] == '-' {
				return core.ErrExchangeContract
			}
			if index == len(host) && numericLabel {
				return core.ErrExchangeContract
			}
			labelStart, numericLabel = index+1, true
			continue
		}
		switch character := host[index]; {
		case character >= '0' && character <= '9':
		case character >= 'a' && character <= 'z', character >= 'A' && character <= 'Z', character == '-':
			numericLabel = false
		default:
			return core.ErrExchangeContract
		}
	}
	return nil
}

// ResolveIP looks up one host through Go's default resolver. It returns 1..16
// unmapped, zone-free, distinct addresses in resolver order. No caching or
// retry occurs; admitting an address for a later DialTCP is caller policy.
// Native lookup errors retain their *net.DNSError beside ErrExchangeTransport.
func ResolveIP(ctx context.Context, request ResolveIPRequest) ([]netip.Addr, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return nil, err
	}
	if err := request.Validate(); err != nil {
		return nil, err
	}
	addresses, err := net.DefaultResolver.LookupNetIP(ctx, "ip", request.Host)
	if err != nil {
		return nil, transportError(err)
	}
	return admitResolvedAddresses(addresses)
}

func admitResolvedAddresses(addresses []netip.Addr) ([]netip.Addr, error) {
	admitted := make([]netip.Addr, 0, min(len(addresses), ResolveIPMaximumAddresses))
next:
	for _, address := range addresses {
		if !address.IsValid() {
			return nil, responseError(errors.New("resolver returned an invalid address"))
		}
		address = address.WithZone("").Unmap()
		for _, seen := range admitted {
			if seen == address {
				continue next
			}
		}
		if len(admitted) == ResolveIPMaximumAddresses {
			return nil, responseError(errors.New("resolver returned more than 16 addresses"))
		}
		admitted = append(admitted, address)
	}
	if len(admitted) == 0 {
		return nil, responseError(errors.New("resolver returned no address"))
	}
	return admitted, nil
}

var _ core.Validatable = ResolveIPRequest{}
