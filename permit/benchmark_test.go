package permit

import (
	"bytes"
	"strconv"
	"testing"
)

// fullPermitFixture signs the shared fixture terms with a full action set so
// membership work runs at the package's declared maximum.
func fullPermitFixture(b *testing.B) (VerifyRequest, Actions) {
	b.Helper()
	request, signer := permitFixture(b)
	values := make([]Action, 0, ActionsMaximumCount)
	for index := range ActionsMaximumCount {
		values = append(values, permitAction(b, "operation-"+strconv.Itoa(index)))
	}
	actions, err := NewActions(values...)
	if err != nil {
		b.Fatalf("NewActions(full set) = %v, want nil", err)
	}
	terms := request.Document.Terms
	terms.Actions = actions
	document, err := Sign(terms, signer)
	if err != nil {
		b.Fatalf("Sign(full set) = %v, want nil", err)
	}
	request.Document = document
	return request, actions
}

var benchmarkVerifiedSink Verified

func BenchmarkVerify(b *testing.B) {
	request, _ := fullPermitFixture(b)
	b.ReportAllocs()
	for b.Loop() {
		verified, err := Verify(request)
		if err != nil {
			b.Fatalf("Verify() = %v, want nil", err)
		}
		benchmarkVerifiedSink = verified
	}
	if terms, err := benchmarkVerifiedSink.Terms(); err != nil || terms.Actions != request.Document.Terms.Actions {
		b.Fatalf("Verify() terms = %v, want signed full action set", err)
	}
}

var benchmarkAllowsSink error

func BenchmarkVerifiedAllows(b *testing.B) {
	request, actions := fullPermitFixture(b)
	verified, err := Verify(request)
	if err != nil {
		b.Fatalf("Verify() = %v, want nil", err)
	}
	last := actions.values[actions.count-1]
	b.ReportAllocs()
	for b.Loop() {
		benchmarkAllowsSink = verified.Allows(last, request.EffectiveAt)
		if benchmarkAllowsSink != nil {
			b.Fatalf("Allows(member) = %v, want nil", benchmarkAllowsSink)
		}
	}
}

func BenchmarkVerifiedAllowsAll(b *testing.B) {
	request, actions := fullPermitFixture(b)
	verified, err := Verify(request)
	if err != nil {
		b.Fatalf("Verify() = %v, want nil", err)
	}
	if count, err := actions.Count(); err != nil || count != ActionsMaximumCount {
		b.Fatalf("Actions.Count() = %d/%v, want %d/nil", count, err, ActionsMaximumCount)
	}
	b.ReportAllocs()
	for b.Loop() {
		benchmarkAllowsSink = verified.AllowsAll(actions, request.EffectiveAt)
		if benchmarkAllowsSink != nil {
			b.Fatalf("AllowsAll(full set) = %v, want nil", benchmarkAllowsSink)
		}
	}
}

var benchmarkDocumentSink Document

func BenchmarkDecode(b *testing.B) {
	request, _ := fullPermitFixture(b)
	var encoded bytes.Buffer
	if err := request.Document.Write(&encoded); err != nil || encoded.Len() == 0 {
		b.Fatalf("Document.Write() = %d bytes/%v, want nonempty/nil", encoded.Len(), err)
	}
	data := encoded.Bytes()
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	for b.Loop() {
		document, err := Decode(bytes.NewReader(data))
		if err != nil {
			b.Fatalf("Decode() = %v, want nil", err)
		}
		benchmarkDocumentSink = document
	}
	if benchmarkDocumentSink != request.Document {
		b.Fatal("Decode() document differs from the signed fixture")
	}
}
