// Package gmail provides a read-only client and typed Bridge server for an
// authorized Google Workspace mailbox. It delegates HTTP to Exchange.
//
// Google publishes the provider contracts for each projection:
//
//   - RefreshRequest → Access: Google's server-side OAuth guide defines the
//     form fields, bearer token, scope and lifetime. Gmail profile supplies
//     emailAddress, which the SDK binds to the configured mailbox before an
//     access handle can be used.
//   - ThreadListRequest → ThreadPage: threads.list admits labelIds, maxResults
//     and pageToken and returns thread IDs and nextPageToken. Google's total is
//     an estimate and is omitted.
//   - ThreadReadRequest → Thread: threads.get in metadata format supplies
//     message IDs, labels, dates and the requested headers. internalDate is
//     a decimal string in milliseconds since the Unix epoch. The SDK checks
//     the outer thread and every message's membership before projection.
//   - MessageReadRequest → Message: messages.get in full format supplies a
//     parsed MIME payload. Gmail's MessagePartBody carries base64url data or a
//     separate attachment ID. The SDK admits inline UTF-8 text/plain only.
//
// The SDK's page, thread, response and text ceilings are local policy. It
// also confines consent to gmail.readonly, refuses redirects, and withholds
// drafts, remote HTML and attachment data.
//
// OAuth: https://developers.google.com/identity/protocols/oauth2/web-server#offline
// Scope: https://developers.google.com/workspace/gmail/api/auth/scopes
// Profile: https://developers.google.com/workspace/gmail/api/reference/rest/v1/users/getProfile
// Thread list: https://developers.google.com/workspace/gmail/api/reference/rest/v1/users.threads/list
// Thread metadata: https://developers.google.com/workspace/gmail/api/reference/rest/v1/users.threads/get
// Message read: https://developers.google.com/workspace/gmail/api/reference/rest/v1/users.messages/get
// Message fields: https://developers.google.com/workspace/gmail/api/reference/rest/v1/users.messages#Message
// MIME body: https://developers.google.com/workspace/gmail/api/reference/rest/v1/users.messages.attachments#MessagePartBody
package gmail
