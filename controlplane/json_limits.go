package controlplane

import "github.com/deliri/primitive/v2026/core"

// documentJSONLimits builds the strict decode limits every document in this
// package shares, differing only in its own byte bound.
//
// One helper rather than a per-document literal: the depth and field allowances
// are a property of the control-plane document family, and a document that
// quietly widened them would accept a shape the rest of the family refuses.
func documentJSONLimits(maximumBytes uint64) (core.StrictJSONLimits, error) {
	maximum, err := core.NewByteCount(maximumBytes)
	if err != nil {
		return core.StrictJSONLimits{}, err
	}
	limits := core.DefaultStrictJSONLimits()
	limits.DocumentMaximumBytes = maximum
	return limits, nil
}
