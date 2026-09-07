## Summary

The local changes tighten real boundaries: idempotency keys are observed and validated before they become transport identity, receive paths zero a document when body custody fails, official-SDK streaming requires a strictly parsed query, response buffering validates merged destination+buffer framing before commit, and stdlib `http.Error`/`Redirect`/`NotFound` writes are observed. Those diffs look correct under the package's custody and replay tests. The remaining defects are in the streaming copy path: a successful read can be dropped when cancellation is observed after `Read` returns, non-seekable `*os.File` sources are refused as if their extent were proven, and `SendReplayBoundSocketJSON` projects the key twice.

## Issues

### Issue 1 -- Severity: bug
- File: stream.go:668
- Description: After a successful `source.Read` (`readErr == nil` and `count > 0`), `progressReader.Read` re-checks the context and, on cancellation, returns `0, err`. `io.CopyBuffer` therefore skips `dst.Write` for that chunk even though the bytes were already consumed from the HTTP body or caller source. The comment at stream.go:661-664 correctly preserves `count` when `Read` itself fails; the success path does the opposite and can under-count `ReceiveStream`/`Download`/`WriteStream` transfers and hide an oversize empty-body probe as cancellation. `copyDownload` already returns the copied count alongside a later context error (stream.go:719); this helper does not.
- Suggestion: If `count > 0`, return that count with the cancellation error (or `count, nil` and let the next `Read` report cancellation) so already-consumed bytes are delivered. Do not zero `count` after a valid read.
- Status: open

### Issue 2 -- Severity: bug
- File: stream.go:411
- Description: `exactUploadSourceExtent` treats any `uploadFileReader` (`Seek`+`Stat`, which `*os.File` implements) as a proven extent. `uploadFileExtent` returns `proven=true` when `Seek` or `Stat` fails, and `newUploadHTTPRequest` then refuses the call (stream.go:332-335). Pipes, terminals, and `os.Stdin` fail `Seek` with `ESPIPE`, so a legitimate O(1) stream is rejected before the network. Unknown readers that do not implement `Seek` correctly fall through as unproven (`return 0, false, nil` at stream.go:386).
- Suggestion: On `Seek`/`Stat` failure, treat the source as unproven (`return 0, false, nil`) rather than as a proven contract violation, matching non-file readers. Only compare remaining size to `ContentLength` when both `Seek` and `Stat` succeed.
- Status: open

### Issue 3 -- Severity: bug
- File: socket.go:176
- Description: `SendReplayBoundSocketJSON` calls `observedIdempotencyKey` to populate request semantics, then `SendReplayBoundJSON` (client.go:227) observes the same body again before encode. The new callback tests treat `IdempotencyKey()` as a hostile projection that may error, panic, or return unset; a second call can disagree with the first, panic after the first succeeded, or apply a side effect twice. Direct `SendReplayBoundJSON` observes once.
- Suggestion: Observe the key once in the socket send, put that value in `RequestSemantics`, and either skip the second observation or compare against the already-observed key without invoking the callback again.
- Status: open

### Issue 4 -- Severity: suggestion
- File: stream.go:685
- Description: `copyDownload`'s panic recover sets `err` but leaves the named `written` result at 0. A panicking destination `Write` can therefore return `ReceivedStream.Bytes == 0` after real bytes were already written. `ReceiveStream` uses `executeRequestBodyOperation` specifically so partial writes stay observable beside a refusal (server.go:350-352).
- Suggestion: Count bytes in a wrapping writer (or capture `io.CopyBuffer`'s count in the recover path) so a recovered panic still reports the destination extent that actually landed.
- Status: open
