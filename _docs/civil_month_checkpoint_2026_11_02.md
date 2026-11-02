# Clamped civil-month targets

Primitive v2026.1.467 adds CivilDate.AddMonthsClamped(int32). It targets
the requested Gregorian month directly and clamps a missing original day
to that month's last day. Jan31 plus two months remains March31; adding
one month twice can instead end on March28. Recurrence callers must choose
the original anchor explicitly. Existing AddCalendar normalization is unchanged.

This operation returns only a date. It cannot silently select a timezone,
DST fold or missing wall time. Kernel must still resolve its exact business
clock and preview the resulting closure before any durable mutation.

Scoped native proof passes48 nodes, including730485 coordinate comparisons
over a complete Gregorian400-year cycle and signed18-month shifts. Removing
the clamp genuinely fails the January31 case; restored failed-only proof
passes both nodes. A3-second semantic fuzz campaign reports34432 executions
and3 new-interesting values, not unique or cumulative coverage.

Scoped vet/staticcheck report zero findings. Witness reports zero findings
in the three changed files, with24 findings in14 byte-identical466 files
remaining open. This is not a whole-package lint pass. The release does not
certify live booking, durable calendar effects or full TimeMatch integration.
