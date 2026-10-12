# Native kernel observation

Hostfacts now returns validated system, release and machine fields from native
kernel APIs. Linux uses the standard library uname syscall and fixed native
fields; Darwin uses the standard library sysctl attributes. Host names and
detailed build descriptions are excluded. Other platforms return the existing
typed Unsupported identity. Cancellation and failed observations return no fact.
Products own redaction and interpretation; no subprocess or PATH lookup is used.

Furnace evidence is retained under
`/data/evidence/raw/primitive/2026-10-12-kernel-native`, with all repositories,
caches, build scratch, temporary files and proof on `/data`. Hostfacts race
proof passed 1,982 terminal events. Byte-domain, native terminator-position and
field-presence domains are exhausted. Independent native uname/syscall agreement,
semantic C-string fuzz and source mutations challenge field binding and malformed
label admission. Earlier failed structural and build attempts are retained.

Linux behavior is proved on Furnace. Darwin and unsupported-platform builds do
not supply native platform execution proof. Global lint/Hammer and unrelated
package migrations remain open; independent acceptance belongs to the user.
