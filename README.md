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

    filelock[filelock] --> core
    filelock --> contextstate
    filestore[filestore] --> core
    filestore --> contextstate
    filestore --> temporal
    hostfacts[hostfacts] --> core
    hostfacts --> contextstate
    temporal[temporal] --> core
    temporal --> contextstate

    exchange[exchange] --> core
    exchange --> contextstate
    exchange --> keygen
    exchange --> temporal
    fuzzfinder[fuzzfinder] --> core
    fuzzfinder --> filestore
    id[id] --> core
    id --> temporal
    lease[lease] --> core
    lease --> temporal
    lease --> attest
    gate[gate] --> core
    gate --> lease
    receipt[receipt] --> core
    receipt --> attest
    receipt --> temporal
    controlwire[controlwire] --> core
    controlwire --> keygen
    controlwire --> exchange
    controlwire --> temporal
    controlplane[controlplane] --> core
    controlplane --> controlwire
    controlplane --> attest
    controlplane --> lease
    controlplane --> temporal
    controlplane --> receipt
    submission[submission] --> core
    submission --> attest
    submission --> chit
    submission --> controlwire
    submission --> id
    submission --> objectstore
    submission --> temporal
    submission --> receipt
    submissionauth[submissionauth] --> core
    submissionauth --> attest
    submissionauth --> controlplane
    submissionauth --> submission
    controlplanetest[controlplanetest] --> core
    controlplanetest --> controlplane
    controlplanetest --> controlwire
    controlplanetest --> lease
    controlplanetest --> receipt
    controlplanetest --> temporal
    process[process] --> core
    process --> contextstate
    process --> temporal
    release[release] --> core
    release --> temporal
    release --> attest
    release --> filestore
    release --> garble
	 release --> controlwire
	 release --> keygen
    release --> process
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

    deploy[deploy] --> core
    deploy --> objectstore
    deploy --> release

    upgrade[upgrade] --> core
    upgrade --> filestore
    upgrade --> hostfacts
    upgrade --> objectstore
    upgrade --> release
    upgrade --> temporal

    distribution[distribution] --> attest
    distribution --> controlwire
    distribution --> core
    distribution --> deploy
    distribution --> objectstore
    distribution --> release
    distribution --> temporal
    distribution --> upgrade

    distributionauth[distributionauth] --> attest
    distributionauth --> controlplane
    distributionauth --> core
    distributionauth --> distribution

    gcsobjects[gcsobjects] --> core
    gcsobjects --> contextstate
    gcsobjects --> temporal
    gcsobjects --> objectstore

    chit[chit] --> attest
    chit --> core
    chit --> controlwire
    chit --> id
    chit --> receipt
    chit --> temporal

    chitauth[chitauth] --> attest
    chitauth --> chit
    chitauth --> controlplane
    chitauth --> core

    retrieval[retrieval] --> attest
    retrieval --> chit
    retrieval --> controlwire
    retrieval --> core
    retrieval --> filestore
    retrieval --> objectstore
    retrieval --> temporal

    retrievalauth[retrievalauth] --> attest
    retrievalauth --> controlplane
    retrievalauth --> core
    retrievalauth --> retrieval

    payment[payment] --> attest
    payment --> core
    payment --> controlwire
    payment --> currency
    payment --> id
    payment --> receipt
    payment --> temporal

    paymentauth[paymentauth] --> attest
    paymentauth --> controlplane
    paymentauth --> core
    paymentauth --> payment

    wiring[wiring] --> core
```

The diagram is the production graph. A package may additionally declare
test-only edges in the same compiler-owned catalog when its real ingress value
cannot be constructed without the package that produces it. A declared test
edge grants no production dependency, counts against the same per-package
coupling ceiling, and is rejected when no test source uses it. `gate` uses
`attest` and `temporal` to prove real signed leases, `submission` uses `exchange`
to prove real provider completions, `submissionauth` uses `chit`,
`controlplanetest`, `controlwire`, `exchange`, and `objectstore` to
prove real credentialed requests and completions, `chitauth` and `paymentauth`
use `controlplanetest`, `controlwire`, and `receipt` to prove real credentialed
catalog queries, `distributionauth` uses `controlplanetest`, `controlwire`,
`release`, and `temporal` to prove real credentialed update and upgrade
requests,
`retrievalauth` uses `controlplanetest` and `controlwire` to prove real
credentialed retrieval requests, `retrieval` uses `exchange` and `receipt` to
prove real streaming transport and authenticated stored evidence,
`process` uses
`testserial` for process-wide isolation, and `deploy` uses `attest`, `exchange`,
and `temporal` to prove a real authenticated manifest and transfer substrate.

## License

Primitive is licensed under the [Mozilla Public License 2.0](LICENSE).

Copyright 2026 Deliri Software Inc., operating as Off Grid Software.
