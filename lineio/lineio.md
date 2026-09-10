# Lineio

Lineio streams LF-delimited input through a fixed Go buffer. A line or stream
can exceed that buffer by any amount. Buffer exhaustion returns another
fragment; it never becomes a line-length refusal.

Construct `Request{Source, BufferBytes}`. The byte count configures working
memory only. It must be positive and representable as a native buffer size.
Go may raise very small requests to its reader minimum; `Reader.Capacity`
reports the actual fixed capacity. Primitive imposes no additional eager
allocation, line-length, or total-stream quota.

`Reader.ReadFragment` delegates to `bufio.Reader.ReadSlice`. It returns a
typed borrowed `Fragment` with exact bytes, including LF and any CR.
`More=true` means the buffer filled before LF and the same line continues.
The completing More=false fragment belongs to the preceding continuation
fragments of the same line, even when it contains only LF. Consumers may process
fragments incrementally; whole-line accumulation is not required.
The next read may overwrite the view. No newline normalization, complete-line
assembly, or complete-stream accumulation occurs inside Primitive.

Always consume returned bytes before handling the accompanying error.
Clean exhaustion is `io.EOF`; an unterminated final fragment may accompany it.
Other failures retain `core.ErrLineIOScan` and the native identity. A terminal
failure remains terminal without reading again. Invalid native reader counts
are refused before Go's buffer uses them.

The caller owns source cleanup, cancellation and any deliberate materialization
of complete product records. A sequential consumer naturally supplies
backpressure by requesting its next fragment only when ready. The buffer does
not grow with a 1 TB or 100 TB stream; processing time grows with bytes read.

The former Scanner/BufferPolicy/MaximumLineBytes contract is removed.
Callers must consume fragments explicitly; there is no compatibility scanner.
