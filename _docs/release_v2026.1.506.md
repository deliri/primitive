# Primitive 2026.1.506

An issued download capability can write a bodyless 303 response with a no-store policy. The signed URL goes directly into the response Location header. Redirects refuse capabilities that require a signed request header other than Host.

Local transport proof checks the exact Location, status, body, cache policy, bounded header size, and refusal when a browser cannot send the signed headers. Removing the host-only rule breaks the intended refusal test.
