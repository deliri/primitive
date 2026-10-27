package cloudbilling

import (
	"bytes"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/currency"
)

func FuzzMoneyGeographyAndQuotaIngress(f *testing.F) {
	seed := Money{Currency: currency.CodeUSD, Units: -1, Nanos: -2}
	if err := seed.Validate(); err != nil {
		f.Fatal(err)
	}
	money, err := seed.MarshalJSON()
	if err != nil {
		f.Fatal(err)
	}
	f.Add(money)
	for kind := GeographyUnspecified; kind <= GeographyMultiRegional; kind++ {
		data, err := kind.MarshalJSON()
		if err != nil {
			f.Fatal(err)
		}
		f.Add(data)
	}
	quota, err := ParseQuotaProject("test-project")
	if err != nil {
		f.Fatal(err)
	}
	f.Add([]byte(quota.String()))
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, data []byte) {
		money := seed
		err := money.UnmarshalJSON(data)
		if err != nil {
			if !errors.Is(err, core.ErrCloudBillingResponse) || money != seed {
				t.Fatalf("money refusal=(%+v,%v), want preserved receiver/typed refusal", money, err)
			}
		} else {
			var independent struct {
				Currency string `json:"currencyCode"`
				Units    string `json:"units"`
				Nanos    int32  `json:"nanos"`
			}
			if err := json.Unmarshal(data, &independent); err != nil {
				t.Fatal(err)
			}
			units := int64(0)
			if independent.Units != "" {
				units, err = strconv.ParseInt(independent.Units, 10, 64)
				if err != nil {
					t.Fatal(err)
				}
			}
			if money.Currency.String() != independent.Currency || money.Units != units || money.Nanos != independent.Nanos || money.Nanos < -999999999 || money.Nanos > 999999999 || money.Units > 0 && money.Nanos < 0 || money.Units < 0 && money.Nanos > 0 {
				t.Fatalf("accepted money differs from exact provider integers: %+v vs %+v", money, independent)
			}
			canonical, err := money.MarshalJSON()
			if err != nil {
				t.Fatal(err)
			}
			var again Money
			if err := again.UnmarshalJSON(canonical); err != nil || again != money {
				t.Fatalf("money fixed point=(%+v,%v), want %+v", again, err, money)
			}
		}
		geography := GeographyRegional
		err = geography.UnmarshalJSON(data)
		if err != nil {
			if !errors.Is(err, core.ErrCloudBillingResponse) || geography != GeographyRegional {
				t.Fatalf("geography refusal=(%v,%v), want preserved receiver", geography, err)
			}
		} else {
			if err := geography.Validate(); err != nil {
				t.Fatal(err)
			}
			canonical, err := geography.MarshalJSON()
			if err != nil {
				t.Fatal(err)
			}
			var again GeographyType
			if err := again.UnmarshalJSON(canonical); err != nil || again != geography {
				t.Fatalf("geography fixed point=(%v,%v), want %v", again, err, geography)
			}
			var token string
			if err := json.Unmarshal(data, &token); err != nil {
				t.Fatal(err)
			}
			want := ""
			switch geography {
			case GeographyUnspecified:
				want = "TYPE_UNSPECIFIED"
			case GeographyGlobal:
				want = "GLOBAL"
			case GeographyRegional:
				want = "REGIONAL"
			case GeographyMultiRegional:
				want = "MULTI_REGIONAL"
			default:
				t.Fatal("accepted geography outside closed provider domain")
			}
			if token != want {
				t.Fatalf("geography token=%q, want %q", token, want)
			}
		}
		quota, err := ParseQuotaProject(string(data))
		if err != nil {
			if !errors.Is(err, core.ErrCloudBillingContract) || quota.String() != "" {
				t.Fatalf("quota refusal=(%v,%v), want zero/contract", quota, err)
			}
		} else {
			if quota.String() != string(data) || quota.String() == "" || strings.IndexFunc(quota.String(), func(r rune) bool { return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-') }) >= 0 {
				t.Fatalf("admitted quota=%q, want exact header-safe selected project", quota.String())
			}
			if err := quota.Validate(); err != nil {
				t.Fatal(err)
			}
			again, err := ParseQuotaProject(quota.String())
			if err != nil || again != quota || !bytes.Equal([]byte(again.String()), data) {
				t.Fatalf("quota fixed point=(%v,%v), want %v", again, err, quota)
			}
		}
	})
}
