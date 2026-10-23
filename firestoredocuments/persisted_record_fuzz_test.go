package firestoredocuments

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"cloud.google.com/go/firestore"
	"github.com/deliri/primitive/v2026/core"
)

type incompatibleRecord struct {
	Owner string `firestore:"owner"`
	Value string `firestore:"value"`
}

func (incompatibleRecord) Validate() error { return nil }

func FuzzPersistedRecordAdmitsExactTypedFacts(f *testing.F) {
	client, _ := nativeClient(f)
	seed := nativeRecord{Owner: "canonical-native-owner", Value: 17}
	if err := seed.Validate(); err != nil {
		f.Fatal(err)
	}
	seedRef := client.Collection("fuzz").Doc(fmt.Sprintf("canonical-seed-%d", os.Getpid()))
	_, err := RunTransaction(f.Context(), TransactionRequest{Client: client, Use: func(ctx context.Context, tx *firestore.Transaction) error {
		return Create(ctx, CreateRequest[nativeRecord]{Reference: seedRef, Transaction: tx, Record: seed})
	}})
	if err != nil {
		f.Fatal(err)
	}
	canonical, err := Get[nativeRecord](f.Context(), GetRequest{Reference: seedRef})
	if err != nil || canonical.Record != seed {
		f.Fatalf("native emitted seed=(%+v,%v), want (%+v,nil)", canonical, err, seed)
	}
	f.Add(canonical.Record.Owner, canonical.Record.Value, false)
	f.Add("", int64(0), false)
	f.Add("wrong-native-type", int64(3), true)
	f.Add(strings.Repeat("a", 16*1024+1), int64(-1), false)
	var ordinal atomic.Uint64
	f.Fuzz(func(t *testing.T, owner string, value int64, wrongType bool) {
		ref := client.Collection("fuzz").Doc(fmt.Sprintf("record-%d-%d", os.Getpid(), ordinal.Add(1)))
		defer func() {
			if _, err := ref.Delete(t.Context()); err != nil {
				t.Errorf("fuzz fixture cleanup=%v, want nil", err)
			}
		}()
		commit, err := RunTransaction(t.Context(), TransactionRequest{Client: client, Use: func(ctx context.Context, tx *firestore.Transaction) error {
			if wrongType {
				return Create(ctx, CreateRequest[incompatibleRecord]{Reference: ref, Transaction: tx, Record: incompatibleRecord{Owner: owner, Value: "not-an-integer"}})
			}
			return Create(ctx, CreateRequest[nativeRecord]{Reference: ref, Transaction: tx, Record: nativeRecord{Owner: owner, Value: value}})
		}})
		if err != nil {
			if !errors.Is(err, core.ErrFirestoreDocumentsContract) || commit != (TransactionResult{}) {
				t.Fatalf("native fixture refusal=(%+v,%v), want typed refusal and zero commitment", commit, err)
			}
			return
		}
		if err := commit.Validate(); err != nil {
			t.Fatal(err)
		}
		got, readErr := Get[nativeRecord](t.Context(), GetRequest{Reference: ref})
		if wrongType || owner == "" {
			if !errors.Is(readErr, core.ErrFirestoreDocumentsContract) || got != (Document[nativeRecord]{}) {
				t.Fatalf("persisted malformed record=(%+v,%v), want typed refusal and zero record", got, readErr)
			}
			return
		}
		want := nativeRecord{Owner: owner, Value: value}
		if readErr != nil || got.Record != want || got.Validate() != nil {
			t.Fatalf("persisted typed facts=(%+v,%v), want (%+v,nil)", got, readErr, want)
		}
	})
}
