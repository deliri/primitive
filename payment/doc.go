// Package payment owns authority-signed payment facts and receipt catalog pages.
// It knows exact identities, currency, settlement time, and service intervals;
// product code owns billing policy and provider selection.
//
// Catalog queries select one Core-owned page window. Continuation does not
// impose a total catalog size limit. Signing owns its entry slice before
// invoking a crypto.Signer; verified accessors return independent page copies.
//
// JSON methods accept caller-owned whole documents without package byte quotas.
// Their allocation follows the actual document, not a preallocated maximum;
// they are not constant-memory stream decoders. The canonical representation
// uses Core's encoding/json/v2 configuration and Attest owns signing mechanics.
package payment
