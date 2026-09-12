# Textrepair

`Prefix(Request{Source, MaximumBytes})` returns an ordered, valid UTF-8 prefix
after discarding malformed source bytes. `MaximumBytes` is a core.ByteLength;
zero returns empty. Construct the extent with core.NewByteLength. A genuine
U+FFFD character is valid text and is preserved. If the next valid rune cannot
fit, the prefix ends: a later smaller rune cannot jump ahead of it.

The operation delegates decoding to unicode/utf8. It does not normalize Unicode,
redact secrets, strip control characters or decide product field limits.
Consumers own those policies. Source and budget are typed in Request, the
package's sole production internal-flow carrier; the architecture inventory
ratchets that role and core's package catalog registers PackageTextRepair.

Output and repair scratch grow with the admitted output extent, not total
source length. Scanning can still traverse all malformed input: this is not
a processing-time cap or streaming reader. A valid prefix can borrow the
original string backing storage without allocation, so callers requiring
independent retention must copy it. A byte budget is not a promise that the
Go builder allocates exactly that number of bytes.

The canonical fuzz seed comes from Prefix. The callback compares exact output
against a separate Go range-based ordered-rune oracle, then proves UTF-8 validity,
byte bounds and idempotence. Fixed tests cover all 256 single-byte inputs at
zero/one/two budgets and named multiwidth, malformed and nonpositive-output
boundaries. They do not claim exhaustive Unicode input coverage or a padded
parser rejection quota; malformed bytes are repaired, not rejected documents.
