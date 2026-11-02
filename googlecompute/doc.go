// Package googlecompute executes single-attempt zonal Compute instance reads,
// start/stop requests and operation reads through the official Google client.
// Callers own credentials, transport, deadlines, fleet policy and polling.
// Provider contracts: https://cloud.google.com/compute/docs/reference/rest/v1/instances
// and https://cloud.google.com/compute/docs/reference/rest/v1/zoneOperations/get.
package googlecompute
