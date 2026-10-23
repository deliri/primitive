Relay copies NearInput to FarOutput and FarInput to NearOutput at once, with
the streams owning the lifetime as with cat, and joins both before returning.
It is admitted as the second declared goroutine owner beside TransferPipe by
the same AST profile: one worker, one bounded completion of its own type, one
join, no loops and, unlike TransferPipe, no native pipe acquisition.

Rows: exact independent payloads, empty streams, an echo row whose upward bytes
depend on downward ones (only concurrent copies finish it), each direction's
write refusal joined beside the other's success, and a far close failure. Every
missing stream is refused before any copy with an unjoined zero result.
Declared mutations must fail their rows by assertion; the serial copy fails the
echo row's backstop.
