package process

import (
	"bufio"
	"io"
	"math"
)

// memoryEvents is a fixed fold of the local cgroup kernel counters.
type memoryEvents struct{ oom, kills uint64 }

// readMemoryEvents consumes flat kernel records with fixed memory, including
// future unknown keys. A missing, duplicate, malformed or truncated known fact
// refuses the entire sample rather than manufacturing zero.
func readMemoryEvents(source io.Reader) (memoryEvents, bool) {
	reader := bufio.NewReaderSize(source, 4096)
	var result memoryEvents
	var seen uint8
	for {
		var key [32]byte
		length := 0
		for {
			b, err := reader.ReadByte()
			if err == io.EOF && length == 0 {
				return result, seen == 3
			}
			if err != nil {
				return memoryEvents{}, false
			}
			if b == ' ' {
				break
			}
			if b < 'a' || b > 'z' {
				if b != '_' {
					return memoryEvents{}, false
				}
			}
			if length < len(key) {
				key[length] = b
			}
			if length <= len(key) {
				length++
			}
		}
		if length == 0 {
			return memoryEvents{}, false
		}
		var value uint64
		digits := false
		for {
			b, err := reader.ReadByte()
			if err != nil {
				return memoryEvents{}, false
			}
			if b == '\n' {
				if !digits {
					return memoryEvents{}, false
				}
				break
			}
			if b < '0' || b > '9' {
				return memoryEvents{}, false
			}
			digit := uint64(b - '0')
			if value > (math.MaxUint64-digit)/10 {
				return memoryEvents{}, false
			}
			value = value*10 + digit
			digits = true
		}
		if length <= len(key) {
			switch string(key[:length]) {
			case "oom":
				if seen&1 != 0 {
					return memoryEvents{}, false
				}
				seen |= 1
				result.oom = value
			case "oom_kill":
				if seen&2 != 0 {
					return memoryEvents{}, false
				}
				seen |= 2
				result.kills = value
			}
		}
	}
}
