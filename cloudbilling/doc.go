// Package cloudbilling owns Google Cloud Billing's ListSKUs wire contract and
// standard Google Application Default Credentials transport, including an
// explicitly selected authorized-user file. One call obtains
// exactly one page: callers choose the endpoint, currency, page size, response
// bounds, deadline and proof sink, and own any pagination or pricing policy.
//
// Provider references:
// https://cloud.google.com/billing/docs/reference/rest/v1/services.skus/list
// https://cloud.google.com/billing/docs/reference/rest/Shared.Types/Money
// https://protobuf.dev/programming-guides/json/
package cloudbilling
