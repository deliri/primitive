# Secretstore

Secretstore owns one product-neutral capability: resolving the current enabled
version of one explicitly named global Google Secret Manager secret through the
official SDK and releasing an exact-version-bound, bounded, redacted,
explicitly destroyable result.

The Google effect leaf is the official Google Cloud Secret Manager Go SDK.
Secretstore verifies the provider's CRC32C before releasing a value and keeps
native provider errors reachable.

It does not own a project, secret, endpoint, deployment, or default. It does not
discover projects or secrets, list versions, accept regional resources, project
numbers, aliases, or numeric input selectors, cache or refresh values, add retries to
the SDK retry policy, persist credentials, or decide how a product uses a secret.


Memory and transport ownership: the official Google SDK materializes one
protobuf response under its native transport configuration. Primitive does not
impose a separate metadata/envelope allowance. This is not an O(1) streaming
protobuf decoder. Secret custody copies only the actual payload length and
rejects payloads above Google's published 64 KiB limit. The projection clears
its owned provider payload on success and refusal; callers own copies returned
by CopyBytes and immutable strings returned by Text. Destroy cannot erase a
caller-owned copy or guarantee removal of SDK/runtime copies.

The provider authenticates the project-ID to project-number resolution.
Primitive verifies canonical numeric response identity, the requested secret
ID, and CRC32C; CRC32C is integrity checking, not an authentication signature.
Access holds the reader lock through the SDK call because SDK Close is excluded
from its concurrent-call guarantee. Caller cancellation releases the call.

Provider contracts: [payload limit](https://docs.cloud.google.com/secret-manager/quotas),
[secret-ID grammar](https://docs.cloud.google.com/secret-manager/docs/reference/rest/v1/projects.secrets/create),
[project-ID grammar](https://docs.cloud.google.com/resource-manager/docs/creating-managing-projects).
