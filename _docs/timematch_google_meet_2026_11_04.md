# Owned Google Meet generation

This is SDK source and controlled-provider proof work, not live Workspace or
TimeMatch launch acceptance. The existing supplied address/HTTPS venue remains
valid. `core.TimeMatchVenueAutomaticVideo` is a request, never a placeholder URL:
its Address and JoinURL must be empty. Only Google currently executes it;
other calendar providers reject it before effects.

`PutEvent` and `UpdateEvent` retain their existing deterministic ownership,
revision, conditional mutation and exact interval checks. Google writes include
`conferenceDataVersion=1` and a `hangoutsMeet` createRequest. The validated
EventID is the requestId: distinct events differ, retry and reschedule keep the
same intent. The observed createRequest must echo that exact requestId and
solution. Existing recover-only remains read-only and requests conference fields
only after the IDs/revisions-only private ownership filter succeeds.

The returned EventReceipt has VenueStatus None for the unchanged supplied-venue
path. Automatic creation returns Pending (event exists, no usable URL), Ready
(the exact owned HTTPS meet.google.com video URI), or Failed (explicit provider
failure, no URL). Missing or inconsistent facts return zero receipt and a typed
indeterminate/conflict error. Ready accepts one video entry, rejects other hosts,
userinfo, explicit ports, queries and fragments, and discards phone/SIP details.
An unresolved automatic venue cannot be written into an invitation.

Applications must retain the original intent and dispatch fact, require Ready
before confirming or delivering a meeting link, and use the same intent with
EventCreateRecoverOnly after a pending or lost reply. Each call is bounded;
there is no SDK polling loop, queue, scheduler, new identity or automatic retry
of a known failed conference request. Application retirement still ends authority.

Verified requirements: current SDK474 has only supplied destination projection;
Google documents asynchronous pending/success/failure, conferenceDataVersion1
and requestId deduplication. Existing EventID's base32hex string is inside the
requestId string domain; Google documents no additional requestId alphabet or
length limit. An application must select the actual consenting professional
Workspace connection/calendar. The identity, enabled Meet policy and live grants
for ase.deliri@offgridsoftware.ca are not verified by this SDK change, and no
professional email is stored or treated as an appointment URL here.

Official contracts read 2026-11-04:
- [Events resource](https://developers.google.com/workspace/calendar/api/v3/reference/events)
- [Insert](https://developers.google.com/workspace/calendar/api/v3/reference/events/insert)
- [Patch](https://developers.google.com/workspace/calendar/api/v3/reference/events/patch)
- [Conference creation guide](https://developers.google.com/workspace/calendar/api/guides/create-events)
