package instagram

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// FuzzListMediaTransientErrorIdentity exercises the public Exchange-backed
// response door. The token code alone determines authentication; a documented
// boolean flag preserves that fact, while malformed or duplicated fields must
// not smuggle an authentication result through strict decoding.
func FuzzListMediaTransientErrorIdentity(f *testing.F) {
	for _, code := range []int{core.InstagramAccessTokenErrorCode - 1, core.InstagramAccessTokenErrorCode, core.InstagramAccessTokenErrorCode + 1} {
		for _, transient := range []bool{false, true} {
			for mutation := uint8(0); mutation < 7; mutation++ {
				f.Add(code, transient, mutation)
			}
		}
	}
	f.Fuzz(func(t *testing.T, code int, transient bool, mutation uint8) {
		wire := metaErrorWire{Error: metaErrorDetailWire{Code: code, IsTransient: transient}}
		if err := wire.Validate(); err != nil {
			t.Fatalf("typed error seed validation = %v, want nil", err)
		}
		canonical, err := json.Marshal(wire)
		if err != nil {
			t.Fatalf("typed error seed marshal = %v, want nil", err)
		}
		body := string(canonical)
		field := `"is_transient":` + strconv.FormatBool(transient)
		if strings.Count(body, field) != 1 {
			t.Fatalf("canonical transient field count = %d, want 1", strings.Count(body, field))
		}
		kind := mutation % 7
		switch kind {
		case 0:
		case 1:
			body = strings.Replace(body, field, `"is_transient":"false"`, 1)
		case 2:
			body = strings.Replace(body, field, `"is_transient":0`, 1)
		case 3:
			body = strings.Replace(body, field, field+`,`+field, 1)
		case 4:
			body = strings.Replace(body, field, field+`,"unexpected":false`, 1)
		case 5:
			body = body[:len(body)-1]
		case 6:
			body = strings.Replace(body, `,`+field, "", 1)
		default:
			t.Fatalf("mutation kind = %d, want an exhaustive modulo domain", kind)
		}
		if kind != 0 && body == string(canonical) {
			t.Fatalf("mutation %d preserved canonical bytes, want a load-bearing change", kind)
		}
		fake := &metaFake{status: http.StatusBadRequest, body: body}
		page, gotErr := fakeClient(t, fake).ListMedia(context.Background(), MediaListRequest{User: testUser(t), Policy: testPolicy(t)})
		wantAuthentication := (kind == 0 || kind == 6) && code == core.InstagramAccessTokenErrorCode
		if errors.Is(gotErr, core.ErrInstagramAuthentication) != wantAuthentication || errors.Is(gotErr, core.ErrInstagramResponse) == wantAuthentication || !errors.Is(gotErr, core.ErrInstagramContract) {
			t.Fatalf("ListMedia error = %v, want exclusive authentication=%v for code=%d mutation=%d", gotErr, wantAuthentication, code, kind)
		}
		if len(page.Media) != 0 || fake.calls.Load() != 1 {
			t.Fatalf("ListMedia failure effects = %d media/%d calls, want 0 media/1 call", len(page.Media), fake.calls.Load())
		}
	})
}
