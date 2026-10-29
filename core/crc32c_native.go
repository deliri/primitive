package core

import "hash/crc32"

// CRC32COf computes a set Castagnoli checksum over borrowed bytes using Go's
// native CRC32 engine. Empty input has a set numeric zero checksum.
func CRC32COf(data []byte) CRC32C {
	return NewCRC32C(crc32.Checksum(data, crc32.MakeTable(crc32.Castagnoli)))
}
