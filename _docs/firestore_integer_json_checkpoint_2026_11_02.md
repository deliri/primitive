# Firestore integer JSON write checkpoint

Primitive v2026.1.463 adds typed `CreateIntegerJSON`, `SetIntegerJSON` and pure `IntegerJSONExtent` to `firestoredocuments`. It includes published compiler462 unchanged.

The actual validated caller JSON codecs define the stored fields. Every number is a signed int64 integer token; fractions, exponent spellings and overflow refuse before queuing a write. Nanoseconds remain exact native INTEGER values. Create/replace return emitted JSON byte counts, never a commit receipt. Caller cancellation is checked before admission and again before queuing native effects. The transaction still owns commit/abort.

Native Anvil proof includes exact integer kinds at nanosecond and int64 boundaries, nested objects/arrays, actual caller codecs, replacement, transaction abort/retry, conflict, cancellation and zero-effect refusals. The final JSONv2 decoder passed95 selected nodes; the final context/pure-helper change passed20 affected nodes. Serial3s semantic fuzz reported198 executions/49interesting. These overlap and are not a unique-coverage count. Genuine integer-to-float controls failed on invalid acceptance and precision loss; exact corrected-source retries passed.

Final focused vet/staticcheck report0 findings. Five Witness findings on private native JSONv2/Firestore map/any interop remain OPEN with an exact owning analyzer request; no suppression or public payload bag was introduced. This release does not claim all-Primitive lint acceptance, live calendar verification, Kernel booking readiness or completed TimeMatch UI/API.
