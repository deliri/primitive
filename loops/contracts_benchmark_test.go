package loops

import (
	"crypto/sha256"
	"encoding/base64"
	"testing"
)

func BenchmarkParseCredential(b *testing.B) {
	value := []byte("0123456789abcdef0123456789abcdef")
	b.ReportAllocs()
	for b.Loop() {
		credential, err := ParseCredential(value)
		if err != nil {
			b.Fatal(err)
		}
		_ = credential.Close()
	}
}

func BenchmarkVerifyRotatedSignatureHeader(b *testing.B) {
	expected := sha256.Sum256([]byte("expected"))
	other := sha256.Sum256([]byte("other"))
	header := "v1," + base64.StdEncoding.EncodeToString(other[:]) + " v1," + base64.StdEncoding.EncodeToString(expected[:])
	b.ReportAllocs()
	for b.Loop() {
		if err := validateSignatureHeader(header); err != nil {
			b.Fatal(err)
		}
		if err := verifySignatureDigest(header, expected[:]); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRouteEndpointRoundTrip(b *testing.B) {
	contact, err := ParseContactIdentifier(ContactIdentifierEmail, "person+tag@example.com")
	if err != nil {
		b.Fatal(err)
	}
	route := Route{Operation: FindContact, Contact: contact}
	b.ReportAllocs()
	for b.Loop() {
		endpoint, err := route.Endpoint()
		if err != nil {
			b.Fatal(err)
		}
		method, err := route.Method()
		if err != nil {
			b.Fatal(err)
		}
		if recovered, err := routeFromTarget(endpoint, method); err != nil || recovered != route {
			b.Fatalf("routeFromTarget() = (%+v, %v), want exact route", recovered, err)
		}
	}
}
