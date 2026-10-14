// Package ai supplies a provider-independent streaming client/server contract
// and native HTTP adapters for OpenAI and Anthropic. Applications supply typed
// payload encoders and response sinks through their bridges. The SDK owns only
// provider routing, credential projection and one native exchange: no retained
// conversation, model policy, body aggregation, retry loop or transfer quota.
//
// Streams and transports are borrowed. HTTPServer owns its credential copy;
// Close joins in-flight calls through a native read/write lock before clearing
// its owned credential. Cancellation remains owned by each caller's context.
// SDK calls execute once. An idempotency promise is not inferred from a header
// that the provider has not documented as a de-duplication contract.
package ai
