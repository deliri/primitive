# id

## What this package is for

id owns the canonical time-ordered identifiers both ends of the stack persist
and exchange: UUIDv7 and ULID. It exists so no product asks a third-party
library for the clock or the random source an identity is built from. The
instant arrives as one `temporal.Observation` and the entropy arrives as
caller-drawn `core.SecretMaterial`, so every identity in the fleet is built
from the same two doors the caller already trusts.

It owns exactly:

- one request shape carrying the observed instant and the drawn entropy;
- UUIDv7 construction, canonical-text admission, and strict JSON round trip;
  and
- ULID construction, canonical-text admission, and strict JSON round trip.

## The two identities

Both are sixteen bytes carrying the same first fact, the observation's Unix
milliseconds in forty-eight big-endian bits, so byte order is time order.
UUIDv7 spends six of its eighty entropy bits on the RFC 9562 version and
variant marks and renders as the thirty-six-byte lowercase dashed form. ULID
keeps all eighty entropy bits and renders as twenty-six uppercase Crockford
base32 bytes whose first byte never exceeds seven. One value, one spelling:
uppercase hex, braces, URN prefixes, lowercase ULIDs, and the ambiguous
Crockford letters are refused, never corrected.

## What it deliberately does not do

- draw entropy or read a clock: keygen and temporal own those effects, and a
  mint door that hid them here would be a second entropy owner;
- monotonic same-millisecond sequencing: ordering inside one millisecond is
  whatever the entropy says, which is honest and stateless;
- product identity vocabulary: a bug or run identifier wraps one of these
  values downstream; or
- accept a foreign spelling for compatibility.

## When it refuses

A request refuses entropy that is not exactly the keygen minimum secret
extent and an observation whose wall projection is invalid. Construction
refuses an instant before the epoch. The forty-eight-bit millisecond ceiling
is a compile-time fact rather than a runtime branch, because temporal's
exact nanosecond domain already fits inside it and a branch no caller can
exercise proves nothing. Parse refuses any text that is not the one
canonical spelling, and the zero value never validates.

## Where it meets the real world

Nowhere, deliberately. This is a pure value package: its inputs are typed
products of temporal and keygen effects the caller already performed, and it
has no effect leaf. The layering law is satisfied above it, at the doors that
produced the inputs.
