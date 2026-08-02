package timeproof

import "encoding/asn1"

const (
	certificateMaximumCount     = 16
	digestAlgorithmMaximumCount = 4
	signedAttributeMaximumCount = 32
	signerMaximumCount          = 1
	refusalMaximumCodeCount     = 8
	refusalStatusTextCount      = 8
	enumJSONMaximumBytes        = 128
	// derConstructed is the DER identifier bit that marks a constructed value.
	derConstructed = 0x20
	// tstInfoVersion is the only TSTInfo version RFC 3161 defines.
	tstInfoVersion = 1
	// cmsSignedDataVersion is required by RFC 5652 because the encapsulated
	// content type is id-ct-TSTInfo rather than id-data.
	cmsSignedDataVersion = 3
	// cmsSignerInfoVersion is required by RFC 5652 for the
	// issuerAndSerialNumber signer identifier this parser accepts.
	cmsSignerInfoVersion = 1
	// generalNameDirectoryNameTag is the X.509 GeneralName CHOICE index for
	// directoryName.
	generalNameDirectoryNameTag = 4
)

// freeTSAPolicyOID is the single source of the reviewed FreeTSA policy
// identity. TimestampPolicy projects its canonical token from this value.
var freeTSAPolicyOID = asn1.ObjectIdentifier{1, 2, 3, 4, 1}

type timestampPolicyContract struct {
	oid       asn1.ObjectIdentifier
	authority Authority
	policy    TimestampPolicy
}

func policyForAuthority(authority Authority) (timestampPolicyContract, error) {
	registry, err := authorityRegistry(authority)
	if err != nil {
		return timestampPolicyContract{}, err
	}
	return timestampPolicyContract{
		oid: registry.policyOID, authority: authority, policy: registry.policy,
	}, nil
}
