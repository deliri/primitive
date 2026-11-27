// Package searchconsole reads Web Performance data and one domain property's
// permission through typed, bounded Search Console operations.
//
// The five action findings need date, page and query, including both ordered
// page/query layouts, and case-sensitive exact page/query filters. Dates are
// inclusive Pacific civil days. This SDK sends the current type field and
// supports Google's final and all data states and auto, byPage and byProperty
// aggregation. Data grouped or filtered by page uses auto.
// The local page ceiling is 2,001 rows and the response ceiling is 2 MiB.
// The provider permits 25,000 rows per request and may return only top data.
// A nonempty page can propose the next offset. Coverage is never guaranteed.
// A missing row is not zero evidence. Explicit first_incomplete_date marks
// provisional data; the typical two-to-three-day delay is not a calendar cutoff.
//
// Query: https://developers.google.com/webmaster-tools/v1/searchanalytics/query
// Data delay and limits: https://developers.google.com/webmaster-tools/v1/how-tos/all-your-data
// Position and metrics: https://support.google.com/webmasters/answer/7042828
// Each call is single-attempt with a caller-owned timeout and no redirects.
// The service-account source observes time through Temporal and mints one
// short-lived bearer per operation. It retains no token beyond that operation.
//
// Project, user, site and load quotas: https://developers.google.com/webmaster-tools/limits
// Sites: https://developers.google.com/webmaster-tools/v1/sites/get
// Permission levels: https://developers.google.com/webmaster-tools/v1/sites
// Scope: https://developers.google.com/webmaster-tools/v1/how-tos/authorizing
// Errors: https://developers.google.com/webmaster-tools/v1/errors
package searchconsole
