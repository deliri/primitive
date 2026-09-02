package timeproof

import "encoding/asn1"

// OID constructors return operation-owned slices. encoding/asn1 represents an
// object identifier as []int, so package variables would expose mutable global
// protocol state to every parser and verifier call.
func oidSignedData() asn1.ObjectIdentifier {
	return asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 7, 2}
}

func oidContentType() asn1.ObjectIdentifier {
	return asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 3}
}

func oidMessageDigest() asn1.ObjectIdentifier {
	return asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 4}
}

func oidSigningTime() asn1.ObjectIdentifier {
	return asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 5}
}

func oidSigningCertificate() asn1.ObjectIdentifier {
	return asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 16, 2, 12}
}

func oidSigningCertificateV2() asn1.ObjectIdentifier {
	return asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 16, 2, 47}
}

func oidTSTInfo() asn1.ObjectIdentifier {
	return asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 16, 1, 4}
}

func oidExtendedKeyUsage() asn1.ObjectIdentifier {
	return asn1.ObjectIdentifier{2, 5, 29, 37}
}

func oidSHA256() asn1.ObjectIdentifier {
	return asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 2, 1}
}

func oidSHA384() asn1.ObjectIdentifier {
	return asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 2, 2}
}

func oidSHA512() asn1.ObjectIdentifier {
	return asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 2, 3}
}

func oidECDSAWithSHA256() asn1.ObjectIdentifier {
	return asn1.ObjectIdentifier{1, 2, 840, 10045, 4, 3, 2}
}

func oidECDSAWithSHA384() asn1.ObjectIdentifier {
	return asn1.ObjectIdentifier{1, 2, 840, 10045, 4, 3, 3}
}

func oidECDSAWithSHA512() asn1.ObjectIdentifier {
	return asn1.ObjectIdentifier{1, 2, 840, 10045, 4, 3, 4}
}

func oidSHA256WithRSA() asn1.ObjectIdentifier {
	return asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 11}
}

func oidSHA384WithRSA() asn1.ObjectIdentifier {
	return asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 12}
}

func oidSHA512WithRSA() asn1.ObjectIdentifier {
	return asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 13}
}

func oidRSAEncryption() asn1.ObjectIdentifier {
	return asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 1}
}

func oidEd25519() asn1.ObjectIdentifier {
	return asn1.ObjectIdentifier{1, 3, 101, 112}
}
