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
