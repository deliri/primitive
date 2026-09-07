# awsidentity before

Evaluated for the filestore defect class (compiler ceiling used as a heap
reservation; Primitive doing O(n) work stdlib should do).

Finding: no such reservation. ParseAudience is a bounded string. Token decode uses signatureBytes, not a 1 MiB ceiling.
