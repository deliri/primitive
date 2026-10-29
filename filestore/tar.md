# Forward TAR writing

`WriteTAR` borrows a synchronous producer and streams each `TAREntry` through
Go's `archive/tar`, optionally followed by gzip. Callers explicitly select the
existing `PathKind`; no unset kind defaults to a regular file.

| Kind | Source | Bytes | LinkTarget |
| --- | --- | --- | --- |
| `PathKindRegularFile` | Required, borrowed | Exact known extent | Zero |
| `PathKindDirectory` | Nil | Zero | Zero |
| `PathKindSymbolicLink` | Nil | Zero | Existing native opaque target |

Other kinds and contradictory fields are refused. Entry paths remain validated
relative names. Permission metadata admits `0000` through `0777`; native file
type and special-mode bits do not belong in that field. Directory entries
preserve empty directories. Symlink targets are written verbatim, including
dangling, relative or absolute targets; they are neither parsed nor followed.
This is encoding, not permission to extract an entry or traverse its target.

The per-entry `ContentIndexEntry` describes body bytes only: directories and
links return the SHA-256 of the empty stream and zero extent. The final archive
observation covers every encoded header, regular body and compression footer.
An entry or producer refusal, cancellation, panic, output error or native
encoder-close failure withdraws that final observation. Partial output remains
caller-owned. Readers remain caller-owned and are consumed synchronously.

Memory retains only the current entry and a reused fixed copy window. A regular
file's size must be known for its TAR header; an unfinished unknown-length
record stream is a different artifact and must not be disguised as a TAR file.

`StreamTAR` uses the same encoder, entry validation and per-entry indexes while
omitting the whole encoded archive's SHA-256. It returns the exact encoded
`ByteLength`, including native closure and compression footer, only on success.
Its fixed counter preserves the signed output extent domain and rejects invalid
or short destination writes. It does not return a fabricated archive digest.
Use `WriteTAR` whenever the complete archive content index is required.
