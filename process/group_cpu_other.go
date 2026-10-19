//go:build !unix

package process

import (
	"context"

	"github.com/deliri/primitive/v2026/core"
)

func observeGroupCPU(context.Context, GroupObservationRequest) (uint64, error) {
	return 0, core.ErrProcessUnsupported
}
