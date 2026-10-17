package core

const (
	// AES256KeyBytes is the native AES-256 key extent, including all-zero keys.
	AES256KeyBytes = 32
	// AESGCMNonceBytes is the native randomly generated GCM nonce extent.
	AESGCMNonceBytes = 12
	// AESGCMTagBytes is the full native GCM authentication tag extent.
	AESGCMTagBytes = 16
	// AESGCMOverheadBytes includes the prepended nonce and authentication tag.
	AESGCMOverheadBytes = AESGCMNonceBytes + AESGCMTagBytes
	// AESGCMMaximumPlaintextBytes is Go's GCM algorithm limit, not a flow quota.
	AESGCMMaximumPlaintextBytes uint64 = ((1 << 32) - 2) * 16
)
