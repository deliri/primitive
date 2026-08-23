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
numbers, aliases, or numeric input selectors, cache or refresh values, retry
provider calls, persist credentials, or decide how a product uses a secret.
