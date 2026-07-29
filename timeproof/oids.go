package timeproof

import (
	"encoding/asn1"
)

var (
	oidSignedData           = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 7, 2}
	oidContentType          = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 3}
	oidMessageDigest        = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 4}
	oidSigningTime          = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 5}
	oidSigningCertificate   = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 16, 2, 12}
	oidSigningCertificateV2 = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 16, 2, 47}
	oidTSTInfo              = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 16, 1, 4}
	oidExtendedKeyUsage     = asn1.ObjectIdentifier{2, 5, 29, 37}
	oidSHA256               = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 2, 1}
	oidSHA384               = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 2, 2}
	oidSHA512               = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 2, 3}
	oidECDSAWithSHA256      = asn1.ObjectIdentifier{1, 2, 840, 10045, 4, 3, 2}
	oidECDSAWithSHA384      = asn1.ObjectIdentifier{1, 2, 840, 10045, 4, 3, 3}
	oidECDSAWithSHA512      = asn1.ObjectIdentifier{1, 2, 840, 10045, 4, 3, 4}
	oidSHA256WithRSA        = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 11}
	oidSHA384WithRSA        = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 12}
	oidSHA512WithRSA        = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 13}
	oidEd25519              = asn1.ObjectIdentifier{1, 3, 101, 112}
)
