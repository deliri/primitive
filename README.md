# Primitive

Primitive is the product-neutral Go primitive layer used to build reliable
command-line tools and services.

Module: `github.com/deliri/primitive/v2026`

First release: `v2026.0.0`

Primitive is a clean rebuild with no compatibility obligation to the archived
implementation. It makes Go's standard library, operating-system primitives,
documented protocols, and official SDKs typed, validated, bounded, streaming,
and easier to compose. The real substrate still does the work.

- [LEDGER.md](LEDGER.md) owns project state.
- [\_docs/testing_protocol.md](_docs/testing_protocol.md) owns test doctrine.
- [\_docs/interviews/](_docs/interviews/) preserves the recon evidence.

## Import flow

An arrow `A --> B` means A must directly import and semantically use B. Every
unshown Primitive sibling import is forbidden. The typed architecture catalog
is executable authority for every landed package's production imports. This
diagram is its public human-readable projection.

```mermaid
flowchart TD
    core[core]

    attest[attest] --> core
    contextstate[contextstate] --> core
    currency[currency] --> core
    garble[garble] --> core
    keygen[keygen] --> core
    testserial[testserial] --> core

    filestore[filestore] --> core
    filestore --> contextstate
    hostfacts[hostfacts] --> core
    hostfacts --> contextstate
    temporal[temporal] --> core
    temporal --> contextstate

    exchange[exchange] --> core
    exchange --> contextstate
    exchange --> temporal
    fuzz[fuzz] --> core
    fuzz --> filestore
    lease[lease] --> core
    lease --> temporal
    lease --> attest
    process[process] --> core
    process --> contextstate
    process --> temporal
    release[release] --> core
    release --> temporal
    release --> attest
    shutdown[shutdown] --> core
    shutdown --> contextstate
    shutdown --> temporal

    objectstore[objectstore] --> core
    objectstore --> contextstate
    objectstore --> temporal
    objectstore --> exchange
    timeproof[timeproof] --> core
    timeproof --> temporal
    timeproof --> keygen
    cloudidentity[cloudidentity] --> core
    cloudidentity --> temporal
    cloudidentity --> exchange

    upgrade[upgrade] --> core
    upgrade --> filestore
    upgrade --> hostfacts
    upgrade --> objectstore
    upgrade --> release
    upgrade --> temporal
```

## License

Primitive is licensed under the [Mozilla Public License 2.0](LICENSE).

Copyright 2026 Deliri Software Inc., operating as Off Grid Software.
