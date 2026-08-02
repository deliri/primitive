package exchange

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

const jsonMediaTypeText = "application/json"

func jsonMediaType() (core.HTTPMediaType, error) {
	mediaType, err := core.ParseHTTPMediaType(jsonMediaTypeText)
	if err != nil {
		return core.HTTPMediaType{}, errors.Join(core.ErrExchangeContract, err)
	}
	return mediaType, nil
}
