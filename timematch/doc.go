// Package timematch owns calendar connection execution for Google Calendar,
// Microsoft Outlook (personal and work/school accounts) and Apple iCloud
// Calendar: provider consent, credential custody, calendar discovery,
// free/busy availability and owned event effects. Every request crosses
// Primitive exchange; no vendor default HTTP client is used.
//
// Applications own business hours, booking policy, persistence, guest
// capabilities and delivery. This package never creates an application
// account, cookie, booking or database record, and it never returns provider
// event titles, attendees or descriptions in availability results.
//
// Official contracts implemented here (read 2026-11-02):
//   - Google OAuth web server flow: https://developers.google.com/identity/protocols/oauth2/web-server
//   - Google Calendar scopes: https://developers.google.com/workspace/calendar/api/auth
//   - Google freeBusy.query: https://developers.google.com/workspace/calendar/api/v3/reference/freebusy/query
//   - Google calendarList.list: https://developers.google.com/workspace/calendar/api/v3/reference/calendarList/list
//   - Google events insert/get/update/delete: https://developers.google.com/workspace/calendar/api/v3/reference/events
//   - Microsoft identity platform auth code flow: https://learn.microsoft.com/en-us/entra/identity-platform/v2-oauth2-auth-code-flow
//   - Microsoft Graph list calendars: https://learn.microsoft.com/en-us/graph/api/user-list-calendars
//   - Microsoft Graph calendarView: https://learn.microsoft.com/en-us/graph/api/calendar-list-calendarview
//   - Microsoft Graph event resource/create/update/delete: https://learn.microsoft.com/en-us/graph/api/resources/event
//   - Apple third-party access and app-specific passwords: https://support.apple.com/en-us/121539 and https://support.apple.com/en-us/102654
//   - CalDAV: RFC 4791, iCalendar: RFC 5545, scheduling: RFC 6638, discovery: RFC 6764, WebDAV: RFC 4918.
//
// Privacy: availability reads busy time only. Google uses freeBusy.query
// (visitor consent: calendar.events.freebusy on the primary calendar only;
// confirming an appointment is a separate calendar.events.owned consent).
// Apple uses the CalDAV free-busy-query report, which returns VFREEBUSY
// periods; a server that refuses it leaves availability Unknown, never a
// wider event download. Microsoft has no free/busy-only delegated permission
// for personal accounts: calendarView under Calendars.ReadBasic is requested
// with $select=start,end,showAs,isCancelled,isAllDay, so subject, body,
// attendees and location are never downloaded; Graph still returns each
// event's id and etag, which are discarded. Event readback is limited to the
// owned deterministic event this package wrote.
//
// Microsoft getSchedule is not used: its documentation marks delegated
// personal Microsoft accounts "Not supported". calendarView supports personal
// and work/school accounts with Calendars.ReadBasic.
//
// Apple's supported-app authorization (support.apple.com/en-us/121539) has no
// published developer onboarding or endpoints; this package does not invent
// one. Apple connections use the documented app-specific password fallback
// over standards CalDAV.
package timematch
