package core

import "math"

// HTTPServerHeaderMaximumBytes is the portable ceiling for Go's int-valued
// http.Server.MaxHeaderBytes. The same typed agreement must fit on 32-bit and
// 64-bit hosts, and leave room for net/http's int64 read-ahead allowance.
// Callers select their operational limit below this mechanical ceiling.
// witness:waiver doctrine/code_form/const_alias -- The shared portable HTTP admission ceiling must track Go math.MaxInt32; copying its literal would duplicate the standard-library contract.
const HTTPServerHeaderMaximumBytes = math.MaxInt32
