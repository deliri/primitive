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

`Characters(ctx, CharacterRequest{Source})` instead publishes one typed UTF-8
character and source position at a time. Go's `text/scanner.Next` owns decoding
and rune-based line/column positions; the byte offset has its own zero-based
type. No token text is collected. An initial BOM is ignored according to Go's
scanner contract; later BOMs remain characters. NUL and malformed UTF-8 are
refused with `core.ErrLineIOScan`.

The source remains borrowed. Cancellation is checked at each character and
source read; the caller must provide an interruptible reader for a blocking
effect. Go's `bufio.Reader.Peek` owns repeated empty-read refusal and the fixed
read-ahead buffer. Provider failures and native invalid-read-count identity
remain available through `errors.Is`. A wrapped EOF is a failure rather than
clean completion. A published prefix is provisional until clean exhaustion.
The consumer can stop synchronously without a worker, queue or complete source
model. There is no line, token or file-size ceiling.

`StreamCharacters(ctx, Request{Source, BufferBytes})` publishes native
`bufio.Reader.ReadRune` observations from a forward-only source. It reuses
`RecordCharacter`: offsets start at zero and byte widths describe the exact
source extent. Initial and later BOMs, NUL, and malformed UTF-8 remain data;
malformed bytes follow Go's RuneError/width-one behavior. The fixed buffer
never grows to fit a token or line. A prefix remains provisional until clean
EOF; a wrapped EOF, another native failure, or cancellation is a refusal.
Consumer backpressure stops observations without reading the remaining stream.
