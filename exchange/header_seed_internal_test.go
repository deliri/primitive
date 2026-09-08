package exchange

import (
	"github.com/deliri/primitive/v2026/core"
	"strings"
)

// The owning nominal projection supplies canonical seed bytes independently of
// the public constructor being mutated. Validate and Value still execute in
// the fuzz setup; refuse-all constructor controls must fail in the callback.
func HeaderValueCanonicalSeedsForTest() []HeaderValue {
	var seeds []HeaderValue
	for _, value := range []string{"", core.HTTPMediaTypeOctetStream().String(), " padded\tvalue ", string([]byte{0x80, 0xff}), strings.Repeat("a", HeaderValueMaximumBytes-1), strings.Repeat("a", HeaderValueMaximumBytes)} {
		seeds = append(seeds, HeaderValue{value: new(value)})
	}
	return seeds
}
