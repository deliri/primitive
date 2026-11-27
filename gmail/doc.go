// Package gmail provides a typed read and send client and Bridge server for an
// authorized Google Workspace mailbox. It delegates HTTP to Exchange.
//
// Google publishes the provider contracts for each projection:
//
//   - DelegatedRequest → Access: the attached runtime authorizes Google IAM
//     Credentials signJwt for one configured signer. A short-lived assertion
//     binds the signer, Workspace subject and Gmail read-only scope. OAuth
//     returns a bearer and expires_in, and Gmail profile binds that bearer to
//     the configured mailbox. The in-memory source shares one renewal across
//     concurrent callers, renews early, and repeats a read once after HTTP 401.
//     A missing IAM or Workspace grant returns core.ErrGoogleDelegationDenied.
//
//   - RefreshRequest → Access: Google's server-side OAuth guide defines the
//     form fields, bearer token, scope and lifetime. Gmail profile supplies
//     emailAddress, which the SDK binds to the configured mailbox before an
//     access handle can be used. Admit exact read-only or exact read-only plus
//     send scopes. Only the second scope set can issue the send capability.
//
//   - ThreadListRequest → ThreadPage: threads.list admits labelIds, maxResults
//     and pageToken and returns thread IDs and nextPageToken. Google's total is
//     an estimate and is omitted.
//
//   - ThreadReadRequest → Thread: threads.get in metadata format supplies
//     message IDs, labels, dates and the requested headers. internalDate is
//     a decimal string in milliseconds since the Unix epoch. The SDK checks
//     the outer thread and every message's membership before projection. To
//     and Cc are bounded lists of typed mailboxes. Bcc is included only when
//     Gmail supplies it on a SENT message from the bound mailbox. Reply-To
//     indicates where replies should go, not the sender or original recipient.
//
//   - MessageReadRequest → Message: messages.get in full format supplies a
//     parsed MIME payload. Gmail's MessagePartBody carries base64url data or a
//     separate attachment ID. The SDK admits inline UTF-8 text/plain only.
//
//   - ConsentRequest → RefreshToken: exchange an owner-collected offline code
//     for the exact read-only and send scope set and verify the Gmail profile.
//
//   - MessagePrepareRequest → PreparedMessage: freeze bounded MIME alternatives,
//     one optional pinned PDF, and the RFC reply chain. MessageSendRequest sends
//     these bytes once as multipart/related metadata plus RFC 5322 media.
//
//   - SentEvidenceRequest → SentEvidence: read exact metadata or search by RFC
//     Message-ID, then require matching Sent label, sender, recipients, thread
//     if supplied, content digest and provider time. No match means no evidence.
//
//   - HistoryListRequest → HistoryPage: read bounded message additions since a
//     mailbox cursor. Preserve continuations and distinguish a reset condition.
//
// The SDK's operation bounds are local policy. It refuses redirects and
// withholds drafts, remote HTML and attachments on the read path.
//
// Mail send: https://developers.google.com/workspace/gmail/api/reference/rest/v1/users.messages/send
// Multipart media: https://developers.google.com/workspace/gmail/api/guides/uploads
// Thread rules: https://developers.google.com/workspace/gmail/api/guides/threads
// Cursor and stale history: https://developers.google.com/workspace/gmail/api/guides/sync
//
// Google consent code: https://developers.google.com/identity/oauth2/web/guides/use-code-model
// OAuth refresh: https://developers.google.com/identity/protocols/oauth2/web-server#offline
// OAuth service accounts: https://developers.google.com/identity/protocols/oauth2/service-account
// Keyless domain delegation: https://docs.cloud.google.com/iam/docs/best-practices-for-managing-service-account-keys#domain-wide-delegation
// Signer: https://docs.cloud.google.com/iam/docs/reference/credentials/rest/v1/projects.serviceAccounts/signJwt
// Workspace grants: https://knowledge.workspace.google.com/admin/apps/control-api-access-with-domain-wide-delegation
// Scope: https://developers.google.com/workspace/gmail/api/auth/scopes
// Profile: https://developers.google.com/workspace/gmail/api/reference/rest/v1/users/getProfile
// Thread list: https://developers.google.com/workspace/gmail/api/reference/rest/v1/users.threads/list
// Thread metadata: https://developers.google.com/workspace/gmail/api/reference/rest/v1/users.threads/get
// Message read: https://developers.google.com/workspace/gmail/api/reference/rest/v1/users.messages/get
// Message fields: https://developers.google.com/workspace/gmail/api/reference/rest/v1/users.messages#Message
// System labels: https://developers.google.com/workspace/gmail/api/guides/labels
// Address and destination headers: https://www.rfc-editor.org/rfc/rfc5322#section-3.6.3
// Sender and reply address semantics: https://www.rfc-editor.org/rfc/rfc5322#section-3.6.2
// Encoded display names: https://www.rfc-editor.org/rfc/rfc2047#section-5
// MIME body: https://developers.google.com/workspace/gmail/api/reference/rest/v1/users.messages.attachments#MessagePartBody
package gmail
