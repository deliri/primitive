//go:build !linux

package process

func observeMemoryCap(owner memoryCapOwner) MemoryCapObservation {
	return MemoryCapObservation{Limit: owner.limit}
}
