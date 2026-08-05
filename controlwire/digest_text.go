package controlwire

import (
	"github.com/deliri/primitive/v2026/core"
)

// parseCanonicalDigestText decodes exact canonical lowercase hexadecimal into
// one Core digest.
//
// The request nonce and the token verifier are both SHA-256 wide and both are
// canonical lowercase on the wire. Decoding them in one place is what keeps the
// two ends of the conversation agreeing on what canonical means: two local
// decoders could drift on case, width, or trailing bytes with nothing failing
// to compile.
//
// Core owns the grammar. This function adds no rule of its own and deliberately
// does not hand-roll a second hexadecimal decoder, because a second grammar is
// exactly how two owners come to disagree about the same bytes. The returned
// error carries Core's identity only; the calling scalar joins its own.
func parseCanonicalDigestText(value string) (core.SHA256Digest, error) {
	var digest core.SHA256Digest
	if err := digest.UnmarshalText([]byte(value)); err != nil {
		return core.SHA256Digest{}, err
	}
	return digest, nil
}
