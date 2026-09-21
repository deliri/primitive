package permit

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

// Inventory is a wiring proof only; behavioral tests own permission evidence.
func TestPermitProductionStructInventory(t *testing.T) {
	t.Parallel()
	roles := []struct {
		name, role string
		found      bool
	}{
		{name: "BuildTransfer", role: "protocol fact: exact build replacement with unchanged signed rights", found: false},
		{name: "BuildTransferIssuance", role: "internal flow: authenticated prior facts and product-selected build", found: false},
		{name: "BuildTransferVerification", role: "internal flow: expected prior facts and exact destination", found: false},
		{name: "VerifiedBuildTransfer", role: "capability wrapper: independently authenticated certificate and unchanged permission", found: false},
		{name: "ReportRequest", role: "protocol fact: signed report and authority-authenticated device nomination", found: false},
		{name: "QuietPeriod", role: "protocol fact: exact excluded transmission interval", found: false},
		{name: "QuietPeriods", role: "protocol fact: immutable ordered exclusions", found: false},
		{name: "TransmissionPolicy", role: "protocol fact: caller-selected transmission exclusions", found: false},
		{name: "ReportAuthorization", role: "protocol fact: mutable authorization without schedule", found: false},
		{name: "SignedReportAuthorization", role: "protocol fact: authenticated current restrictions", found: false},
		{name: "ReportPermissionResponse", role: "protocol fact: current authorization plus single issued carrier", found: false},
		{name: "ReportHead", role: "internal flow: caller-supplied sequence and digest comparison", found: false},
		{name: "ReportScope", role: "protocol fact: tenant and epoch binding", found: false},
		{name: "ReportEvidence", role: "protocol fact: evidence digest and extent", found: false},
		{name: "ReportPayload", role: "protocol fact: signed additive source facts", found: false},
		{name: "ProjectPermission", role: "protocol fact: initial scope and schedule", found: false},
		{name: "ReportAcknowledgment", role: "sealed projection: committed report identity and next schedule", found: false},
		{name: "SignedReport", role: "protocol fact: device-attested report", found: false},
		{name: "SignedProjectPermission", role: "protocol fact: authority-attested initial schedule", found: false},
		{name: "SignedReportAcknowledgment", role: "sealed projection: authority-attested committed report", found: false},
		{name: "ReportSchedule", role: "protocol fact: caller-selected recurring timing bounds", found: false},
		{name: "ReportOccurrence", role: "internal flow: authorization-intersected occurrence", found: false},
		{name: "ReportTiming", role: "internal flow: supplied observation and current authorization", found: false},
		{name: "RegistrationResponse", role: "protocol fact: enrollment and permission bound to one response", found: false},
		{name: "CheckInResponse", role: "protocol fact: usage response and permission bound to one response", found: false},
		{name: "Action", role: "protocol fact: opaque canonical action identity", found: false},
		{name: "Actions", role: "protocol fact: immutable bounded action set", found: false},
		{name: "Terms", role: "protocol fact: exact server-issued opaque action and validity terms", found: false},
		{name: "Document", role: "protocol fact: terms bound to Primitive attestation", found: false},
		{name: "VerifyRequest", role: "internal flow: independently supplied verification context", found: false},
		{name: "Verified", role: "capability wrapper: authenticated terms for one verification instant", found: false},
	}
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("ReadDir = %v, want nil", err)
	}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), entry.Name(), nil, 0)
		if err != nil {
			t.Fatalf("parse %s = %v, want nil", entry.Name(), err)
		}
		ast.Inspect(file, func(node ast.Node) bool {
			spec, ok := node.(*ast.TypeSpec)
			if !ok {
				return true
			}
			if _, ok := spec.Type.(*ast.StructType); !ok {
				return true
			}
			found := false
			for i := range roles {
				if roles[i].name == spec.Name.Name && roles[i].role != "" {
					found, roles[i].found = true, true
				}
			}
			if !found {
				t.Errorf("production struct %s classified = false, want intentional role", spec.Name.Name)
			}
			return true
		})
	}
	for _, role := range roles {
		if !role.found {
			t.Errorf("inventory struct %s declaration = absent, want present", role.name)
		}
	}
}

// Each external representation boundary names its semantic fuzz oracle.
func TestPermitExternalDoorInventory(t *testing.T) {
	t.Parallel()
	doors := []struct{ name, file, target string }{
		{name: "BuildTransfer.UnmarshalJSON", file: "build_transfer_test.go", target: "FuzzBuildTransferSemanticClosure"},
		{name: "ReportRequest.UnmarshalJSON", file: "report_request_test.go", target: "FuzzReportRequestSemanticClosure"},
		{name: "QuietPeriods.UnmarshalJSON", file: "report_authorization_test.go", target: "FuzzReportPermissionResponse"},
		{name: "SignedReportAuthorization.UnmarshalJSON", file: "report_authorization_test.go", target: "FuzzReportPermissionResponse"},
		{name: "ReportPermissionResponse.UnmarshalJSON", file: "report_authorization_test.go", target: "FuzzReportPermissionResponse"},
		{name: "ReportDomain.ParseCanonicalText", file: "report_signed_test.go", target: "FuzzReportDomain"},
		{name: "SignedReport.UnmarshalJSON", file: "report_signed_test.go", target: "FuzzReportSignedSemanticClosure"},
		{name: "SignedProjectPermission.UnmarshalJSON", file: "report_signed_test.go", target: "FuzzReportSignedSemanticClosure"},
		{name: "SignedReportAcknowledgment.UnmarshalJSON", file: "report_signed_test.go", target: "FuzzReportSignedSemanticClosure"},
		{name: "ParseAction", file: "action_test.go", target: "FuzzParseActionSemanticClosure"},
		{name: "Action.UnmarshalJSON", file: "action_test.go", target: "FuzzActionSemanticClosure"},
		{name: "Actions.UnmarshalJSON", file: "action_test.go", target: "FuzzActionsSemanticClosure"},
		{name: "Decode", file: "document_test.go", target: "FuzzPermitDecodeSignedSemanticClosure"},
		{name: "Domain.ParseCanonicalText", file: "document_test.go", target: "FuzzPermitDomainText"},
		{name: "Revision.UnmarshalJSON", file: "document_test.go", target: "FuzzPermitRevisionJSON"},
		{name: "RegistrationResponse.UnmarshalJSON", file: "response_fuzz_test.go", target: "FuzzRegistrationResponseSemanticClosure"},
		{name: "CheckInResponse.UnmarshalJSON", file: "response_fuzz_test.go", target: "FuzzCheckInResponseSemanticClosure"},
	}
	for _, door := range doors {
		found := false
		file, err := parser.ParseFile(token.NewFileSet(), door.file, nil, 0)
		if err != nil {
			t.Fatalf("parse tests = %v, want nil", err)
		}
		for _, decl := range file.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == door.target {
				found = true
			}
		}
		if !found {
			t.Errorf("external door %s oracle %s found = false, want true", door.name, door.target)
		}
	}
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("ReadDir = %v, want nil", err)
	}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), entry.Name(), nil, 0)
		if err != nil {
			t.Fatalf("parse %s = %v, want nil", entry.Name(), err)
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || !externalRepresentationDoor(fn.Name.Name) {
				continue
			}
			name := fn.Name.Name
			if fn.Recv != nil {
				receiver := fn.Recv.List[0].Type
				if pointer, ok := receiver.(*ast.StarExpr); ok {
					receiver = pointer.X
				}
				if identifier, ok := receiver.(*ast.Ident); ok {
					name = identifier.Name + "." + name
				}
			}
			found := false
			for _, door := range doors {
				found = found || door.name == name
			}
			if !found {
				t.Errorf("external door %s fuzz inventory = absent, want named semantic oracle", name)
			}
		}
	}

}

// Only conventional public representation ingress names are discovered here.
// Constructors admitting external material under other names need review.
func externalRepresentationDoor(name string) bool {
	for _, prefix := range []string{"Parse", "Decode", "Read", "Load", "Replay", "Unmarshal"} {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

func TestExternalRepresentationDoorMatcher(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		want bool
	}{
		{name: "Decode", want: true}, {name: "ParseCanonicalText", want: true}, {name: "UnmarshalJSON", want: true},
		{name: "ReadDocument", want: true}, {name: "LoadDocument", want: true}, {name: "Replay", want: true},
		{name: "MarshalJSON", want: false}, {name: "Validate", want: false}, {name: "decodeInternal", want: false},
	} {
		got := externalRepresentationDoor(tc.name)
		if got != tc.want {
			t.Errorf("externalRepresentationDoor(%q) = %t, want %t", tc.name, got, tc.want)
		}
	}
}
