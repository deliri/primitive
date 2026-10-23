Relay copies NearInput to FarOutput and FarInput to NearOutput at once, with
the streams owning the lifetime as with cat. Each output is owned and closed
as soon as its copy ends, before joining the opposite copy. This lets a near
peer receive EOF and then finish its input, rather than waiting circularly.
Both copies and both closes join before return.
It is admitted as the second declared goroutine owner beside TransferPipe by
the same AST profile: one worker, one bounded completion of its own type, one
join, no loops and, unlike TransferPipe, no native pipe acquisition.

Rows: exact independent payloads, empty streams, an echo row whose upward bytes
depend on downward ones (only concurrent copies finish it), each direction's
write refusal joined beside the other's success, independent and simultaneous
close failures, and a real pipe whose upward EOF depends on downward close.
Every absent or typed-nil stream is refused through Core's ReaderIsNil and
WriterIsNil before any copy or close, with an exactly zero result.
Declared mutations must fail their rows by assertion; the serial copy fails the
echo row's backstop.
