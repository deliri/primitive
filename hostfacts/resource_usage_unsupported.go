//go:build !linux && !darwin

package hostfacts

import (
	"context"
	"github.com/deliri/primitive/v2026/core"
)

func observeResourceUsage(context.Context, ResourceUsageRequest) (ResourceUsage, error) {
	return ResourceUsage{}, core.ErrHostFactsUnsupported
}
