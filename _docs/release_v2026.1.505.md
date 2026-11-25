# Primitive 2026.1.505

GCS can issue a short-lived download capability for one object generation. The official V4 signer includes the generation in the signed query. A later replacement at the same object name cannot change the object version selected by that capability.

Focused tests verify the admitted generation domain and the canonical query sent to a local IAM signing peer. Changing the generation changes the signed digest. The existing object-name capability still signs its original query. This proof uses a local peer. Live GCS and IAM acceptance remain unverified.
