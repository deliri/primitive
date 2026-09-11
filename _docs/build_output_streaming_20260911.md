# Release build output policy

BuildProcessRequest now carries process.OutputPolicy directly. Its former
OutputLimit field forced every caller into bounded output, even when the caller
supplied streaming destinations. PrepareBuildProcess now validates and preserves
the exact typed policy. There is no compatibility field or alternate constructor.
The process owner retains mechanical validation and execution; consumers choose
whether an operation requests streaming or an explicit bound.

Local evidence is retained under
/private/tmp/peachfuzz-state-readonly-evidence. The base revision is
e82d2cea44fdaef5c534478abd55e96bf25a6718. Each attempt retains the exact source
patch, command, toolchain, cache posture, output and exit status.

release-output-policy-compiler-red records that the old request could not express
the typed policy. release-output-policy-green passes streaming, explicit bounded
output, zero mode, contradictory streaming extent, and missing bounded extent.
The test exercises actual repository/tool verification and process preparation;
it does not claim to execute a compiler build. The deliberate forced-bounded
mutation failed release-output-forced-bounded-mutation-red and was discarded.
This pins preservation of the caller's policy without a second process runtime.

These local facts are not independent acceptance. The final repository gate
remains deferred to the end of the authorized conversion.
