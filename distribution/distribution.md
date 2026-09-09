# Distribution

Distribution is the product-neutral agreement joining a software release
authority to a release producer or installed tool.

It owns signed publication requests, fixed release-object upload grants,
provider-evidence completions, installed-build update requests and responses,
and exact-candidate download grants. It projects authenticated agreements into
the existing Deploy and Upgrade capabilities without reading artifact bytes.

It does not own HTTP routes, accounts, installation credentials, commercial or
channel policy, provider credential creation, object naming, release building,
transfer execution, installation trials, persistence, retries, or a lifecycle
state machine. Those decisions and effects remain with the caller and the
Primitive package that already owns them.

## Request and completion boundaries

CommitRequest accepts the concrete PublicationRequestPayload, UpdateRequestPayload
and UpgradeRequestPayload value types. RequestCommitment rejects other signing
domains. Each payload owns validation and bounded canonical encoding.

Completion verification requires all eight transfer evidence slots to identify
the exact upload capability in the signed grant. Deploy preserves that identity
by passing the capability through Objectstore.Upload. Content equality alone
does not identify the granted destination.

Upgrade stage projection validates the supplied root/directory identity through
Filestore before returning a StageRequest. Refusal returns a zero stage.

See [the September 9 review report](../_docs/distribution_upgrade_20260909.md)
for the hostile boundary inventory, measurements and evidence limitations.
