# TimeMatch phase matching correctness checkpoint

Primitive v2026.1.465 extends MatchGroup with hard phase constraints. Offset and OffsetBefore express starts after or before the visit; buffers and complete search-window fit remain exact. Resources do not count as attendees or removable people. Constraint-only matching supports resource discovery without invented participants.

Behavioral proof includes independent 4,096-case and signed 8,448-case point-occupancy oracles, half-open processing gaps, pre/post-visit segments, unknown availability, signed extrema, source immutability and cancellation. Offset, hard-removal and signed-offset omission controls fail; exact restored leaves pass. Three-second semantic fuzzing reports execution facts rather than unique coverage. The earlier whole-service feature-gap refusal and non-detecting singleton control remain qualified, not presented as old matcher bugs.

Final affected vet and Witness are clean at981b7093; staticcheck is clean at0e8d1ae7, with only two test failure messages changed afterward. Original source-bound receipts are retained outside Git.

This is an SDK correctness checkpoint. Kernel integration, durable calendar/booking transactions, live providers, browser workflows, travel, insights and profiled performance remain separate. The d635 module-cache capability is not included and remains open.
