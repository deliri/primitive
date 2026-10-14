This directory contains `lib/time/zoneinfo.zip` from Go 1.27.2. It is authored
runtime data used through Go's `archive/zip` and `time.LoadLocationFromTZData`.
The adjacent LICENSE is Go's BSD license.

Exact archive member names define the admitted zone names. Embedded TZif data
keeps admission and calendar execution independent of the host filesystem,
`ZONEINFO`, and the host's timezone database. Replacing the archive is a source
change; callers reuse one `temporal.Location` rather than building a cache.

Sources: https://go.dev/src/time/zoneinfo_read.go and
https://go.dev/src/time/tzdata/tzdata.go.
