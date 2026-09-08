package core

// POSIXAllocationBlockBytes is the byte unit of Stat_t.Blocks on the Unix
// platforms supported by Filestore. It is independent of the filesystem's
// transfer block size and allocation granularity.
const POSIXAllocationBlockBytes uint64 = 512
