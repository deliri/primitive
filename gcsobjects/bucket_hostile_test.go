package gcsobjects

import (
	"context"
	json "encoding/json/v2"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"cloud.google.com/go/storage"
	"github.com/deliri/primitive/v2026/core"
	"google.golang.org/api/option"
	storageapi "google.golang.org/api/storage/v1"
)

func TestGCSProjectIDHostileBoundaryTable(t *testing.T) {
	t.Parallel()

	cases := []struct {
		wantErr error
		name    string
		input   string
		want    string
	}{
		{name: "exact minimum six bytes", input: "a12345", want: "a12345"},
		{name: "one above minimum", input: "a123456", want: "a123456"},
		{name: "lowercase letters", input: "projectalpha", want: "projectalpha"},
		{name: "internal hyphen", input: "project-alpha", want: "project-alpha"},
		{name: "multiple internal hyphens", input: "project-alpha-7", want: "project-alpha-7"},
		{name: "digits after initial letter", input: "p123456789", want: "p123456789"},
		{name: "alternating letters and digits", input: "p1r2o3j4", want: "p1r2o3j4"},
		{name: "one below maximum", input: "p" + strings.Repeat("a", GCSProjectIDMaximumBytes-2), want: "p" + strings.Repeat("a", GCSProjectIDMaximumBytes-2)},
		{name: "exact maximum", input: "p" + strings.Repeat("a", GCSProjectIDMaximumBytes-1), want: "p" + strings.Repeat("a", GCSProjectIDMaximumBytes-1)},
		{name: "letter final after internal digit", input: "project7a", want: "project7a"},
		{name: "empty", wantErr: core.ErrObjectStoreContract},
		{name: "one byte", input: "a", wantErr: core.ErrObjectStoreContract},
		{name: "one below minimum", input: strings.Repeat("a", GCSProjectIDMinimumBytes-1), wantErr: core.ErrObjectStoreContract},
		{name: "one above maximum", input: "p" + strings.Repeat("a", GCSProjectIDMaximumBytes), wantErr: core.ErrObjectStoreContract},
		{name: "starts with digit", input: "1project", wantErr: core.ErrObjectStoreContract},
		{name: "starts with hyphen", input: "-project", wantErr: core.ErrObjectStoreContract},
		{name: "ends with hyphen", input: "project-", wantErr: core.ErrObjectStoreContract},
		{name: "uppercase", input: "Project", wantErr: core.ErrObjectStoreContract},
		{name: "underscore", input: "project_id", wantErr: core.ErrObjectStoreContract},
		{name: "dot", input: "project.id", wantErr: core.ErrObjectStoreContract},
		{name: "slash", input: "project/id", wantErr: core.ErrObjectStoreContract},
		{name: "space", input: "project id", wantErr: core.ErrObjectStoreContract},
		{name: "newline", input: "project\nid", wantErr: core.ErrObjectStoreContract},
		{name: "multibyte", input: "proje¢t", wantErr: core.ErrObjectStoreContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := ParseGCSProjectID(tc.input)
			if tc.wantErr != nil {
				if !errors.Is(gotErr, tc.wantErr) || got != (GCSProjectID{}) {
					t.Fatalf("ParseGCSProjectID(%q) = (%q, %v), want zero and errors.Is %v",
						tc.input, got.String(), gotErr, tc.wantErr)
				}
				return
			}
			if gotErr != nil || got.String() != tc.want || got.Validate() != nil {
				t.Fatalf("ParseGCSProjectID(%q) = (%q, %v), want (%q, nil)",
					tc.input, got.String(), gotErr, tc.want)
			}
		})
	}
}

func TestGCSLocationHostileBoundaryTable(t *testing.T) {
	t.Parallel()

	cases := []struct {
		wantErr error
		name    string
		input   string
	}{
		{name: "minimum one byte", input: "a"},
		{name: "uppercase multi-region", input: "US"},
		{name: "europe multi-region", input: "EU"},
		{name: "asia multi-region", input: "ASIA"},
		{name: "lowercase region", input: "us-central1"},
		{name: "uppercase digit token", input: "NAM4"},
		{name: "internal single hyphen", input: "northamerica-northeast2"},
		{name: "mixed case nominal token", input: "Region-7"},
		{name: "one below maximum", input: strings.Repeat("a", GCSLocationMaximumBytes-1)},
		{name: "exact maximum", input: strings.Repeat("a", GCSLocationMaximumBytes)},
		{name: "empty", wantErr: core.ErrObjectStoreContract},
		{name: "one above maximum", input: strings.Repeat("a", GCSLocationMaximumBytes+1), wantErr: core.ErrObjectStoreContract},
		{name: "starts hyphen", input: "-region", wantErr: core.ErrObjectStoreContract},
		{name: "ends hyphen", input: "region-", wantErr: core.ErrObjectStoreContract},
		{name: "underscore", input: "us_central1", wantErr: core.ErrObjectStoreContract},
		{name: "slash", input: "us/central1", wantErr: core.ErrObjectStoreContract},
		{name: "dot", input: "us.central1", wantErr: core.ErrObjectStoreContract},
		{name: "space", input: "us central1", wantErr: core.ErrObjectStoreContract},
		{name: "newline", input: "us\ncentral1", wantErr: core.ErrObjectStoreContract},
		{name: "carriage return", input: "us\rcentral1", wantErr: core.ErrObjectStoreContract},
		{name: "multibyte", input: "région", wantErr: core.ErrObjectStoreContract},
		{name: "leading tab", input: "\tregion", wantErr: core.ErrObjectStoreContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := ParseGCSLocation(tc.input)
			if tc.wantErr != nil {
				if !errors.Is(gotErr, tc.wantErr) || got != (GCSLocation{}) {
					t.Fatalf("ParseGCSLocation(%q) = (%q, %v), want zero and errors.Is %v",
						tc.input, got.String(), gotErr, tc.wantErr)
				}
				return
			}
			if gotErr != nil || got.String() != tc.input || got.Validate() != nil {
				t.Fatalf("ParseGCSLocation(%q) = (%q, %v), want exact input and nil",
					tc.input, got.String(), gotErr)
			}
		})
	}
}

func TestGCSNamespaceExhaustsEntireByteDomain(t *testing.T) {
	t.Parallel()

	admitted := 0
	for value := 0; value <= 255; value++ {
		namespace := GCSNamespace(value)
		if namespace.Validate() != nil {
			if namespace.IsValid() || namespace.String() != "" {
				t.Fatalf("GCSNamespace(%d) refused but exposes valid state or diagnostic", value)
			}
			continue
		}
		admitted++
		if !namespace.IsValid() || namespace.String() == "" {
			t.Fatalf("GCSNamespace(%d) admitted without closed validity and diagnostic", value)
		}
	}
	if admitted != 2 {
		t.Fatalf("admitted GCS namespaces = %d, want exact flat and hierarchical pair", admitted)
	}
}

func TestGCSBucketCreateRequestHostilePolicyTable(t *testing.T) {
	t.Parallel()

	base := bucketCreateFixture(t, GCSNamespaceFlat)
	valid := []struct {
		name    string
		request GCSBucketCreateRequest
	}{
		{name: "ordinary flat namespace", request: base},
		{name: "ordinary hierarchical namespace", request: bucketCreateFixture(t, GCSNamespaceHierarchical)},
		{name: "minimum project extent", request: GCSBucketCreateRequest{
			Project: parsedGCSProject(t, "a12345"), Bucket: base.Bucket,
			Location: base.Location, Namespace: GCSNamespaceFlat,
		}},
		{name: "maximum project extent", request: GCSBucketCreateRequest{
			Project: parsedGCSProject(t, "p"+strings.Repeat("a", GCSProjectIDMaximumBytes-1)),
			Bucket:  base.Bucket, Location: base.Location, Namespace: GCSNamespaceFlat,
		}},
		{name: "minimum location extent", request: GCSBucketCreateRequest{
			Project: base.Project, Bucket: base.Bucket,
			Location: parsedGCSLocation(t, "a"), Namespace: GCSNamespaceFlat,
		}},
		{name: "maximum location extent", request: GCSBucketCreateRequest{
			Project: base.Project, Bucket: base.Bucket,
			Location:  parsedGCSLocation(t, strings.Repeat("a", GCSLocationMaximumBytes)),
			Namespace: GCSNamespaceFlat,
		}},
		{name: "minimum bucket extent", request: GCSBucketCreateRequest{
			Project: base.Project, Bucket: parsedGCSBucket(t, "abc"),
			Location: base.Location, Namespace: GCSNamespaceFlat,
		}},
		{name: "dotted bucket maximum domain", request: GCSBucketCreateRequest{
			Project: base.Project, Bucket: parsedGCSBucket(t, strings.Repeat("a", 63)+"."+strings.Repeat("b", 63)),
			Location: base.Location, Namespace: GCSNamespaceFlat,
		}},
		{name: "multi-region location", request: GCSBucketCreateRequest{
			Project: base.Project, Bucket: base.Bucket,
			Location: parsedGCSLocation(t, "US"), Namespace: GCSNamespaceFlat,
		}},
		{name: "numeric region suffix", request: GCSBucketCreateRequest{
			Project: base.Project, Bucket: base.Bucket,
			Location: parsedGCSLocation(t, "us-central1"), Namespace: GCSNamespaceHierarchical,
		}},
	}
	for _, tc := range valid {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if gotErr := tc.request.Validate(); gotErr != nil {
				t.Fatalf("GCSBucketCreateRequest.Validate(%s) error = %v, want nil", tc.name, gotErr)
			}
		})
	}
	invalid := []struct {
		name    string
		request GCSBucketCreateRequest
	}{
		{name: "zero request"},
		{name: "missing project", request: GCSBucketCreateRequest{Bucket: base.Bucket, Location: base.Location, Namespace: base.Namespace}},
		{name: "missing bucket", request: GCSBucketCreateRequest{Project: base.Project, Location: base.Location, Namespace: base.Namespace}},
		{name: "missing location", request: GCSBucketCreateRequest{Project: base.Project, Bucket: base.Bucket, Namespace: base.Namespace}},
		{name: "missing namespace", request: GCSBucketCreateRequest{Project: base.Project, Bucket: base.Bucket, Location: base.Location}},
		{name: "missing project and bucket", request: GCSBucketCreateRequest{Location: base.Location, Namespace: base.Namespace}},
		{name: "missing bucket and location", request: GCSBucketCreateRequest{Project: base.Project, Namespace: base.Namespace}},
		{name: "missing project and namespace", request: GCSBucketCreateRequest{Bucket: base.Bucket, Location: base.Location}},
		{name: "first future namespace", request: GCSBucketCreateRequest{Project: base.Project, Bucket: base.Bucket, Location: base.Location, Namespace: gcsNamespaceLimit}},
		{name: "maximum future namespace", request: GCSBucketCreateRequest{Project: base.Project, Bucket: base.Bucket, Location: base.Location, Namespace: GCSNamespace(255)}},
	}
	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if gotErr := tc.request.Validate(); !errors.Is(gotErr, core.ErrObjectStoreContract) {
				t.Fatalf("GCSBucketCreateRequest.Validate(%s) error = %v, want errors.Is %v",
					tc.name, gotErr, core.ErrObjectStoreContract)
			}
		})
	}
}

func TestGCSBucketProvisioningLayerTriadUsesOfficialSDKAndPreservesExactPolicy(t *testing.T) {
	t.Parallel()

	requests := []GCSBucketCreateRequest{
		bucketCreateFixture(t, GCSNamespaceFlat),
		bucketCreateFixture(t, GCSNamespaceHierarchical),
	}
	for _, request := range requests {
		t.Run(request.Namespace.String()+" namespace", func(t *testing.T) {
			t.Parallel()

			var received storageapi.Bucket
			client := bucketTestClient(t, http.HandlerFunc(func(writer http.ResponseWriter, incoming *http.Request) {
				if incoming.Method != http.MethodPost {
					writer.WriteHeader(http.StatusMethodNotAllowed)
					return
				}
				if err := json.UnmarshalRead(incoming.Body, &received); err != nil {
					t.Errorf("provider request decode error = %v, want nil", err)
					writer.WriteHeader(http.StatusBadRequest)
					return
				}
				writer.Header().Set("Content-Type", "application/json")
				if err := json.MarshalWrite(writer, received); err != nil {
					t.Errorf("provider response encode error = %v, want nil", err)
				}
			}))
			got, gotErr := CreateBucket(context.Background(), client, request)
			if gotErr != nil || got.Validate() != nil || got.Project() != request.Project ||
				got.Bucket() != request.Bucket || got.Location() != request.Location ||
				got.Namespace() != request.Namespace {
				t.Fatalf("CreateBucket(%v) = (%v, %v), want exact provisioning and nil",
					request.Namespace, got, gotErr)
			}
			wantHierarchical := request.Namespace == GCSNamespaceHierarchical
			gotHierarchical := received.HierarchicalNamespace != nil && received.HierarchicalNamespace.Enabled
			if received.Name != request.Bucket.String() || received.Location != request.Location.String() ||
				gotHierarchical != wantHierarchical {
				t.Fatalf("provider bucket request = (%q, %q, hierarchical %t), want (%q, %q, %t)",
					received.Name, received.Location, gotHierarchical,
					request.Bucket.String(), request.Location.String(), wantHierarchical)
			}
		})
	}

	t.Run("negative provider conflict returns no provisioning evidence", func(t *testing.T) {
		t.Parallel()

		client := bucketTestClient(t, http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			writer.WriteHeader(http.StatusConflict)
		}))
		got, gotErr := CreateBucket(context.Background(), client, bucketCreateFixture(t, GCSNamespaceFlat))
		if !errors.Is(gotErr, core.ErrObjectStoreConflict) || got != (GCSBucketProvisioning{}) {
			t.Fatalf("CreateBucket(conflict) = (%v, %v), want zero and errors.Is %v",
				got, gotErr, core.ErrObjectStoreConflict)
		}
	})

	t.Run("neutral zero request and canceled context create no provisioning evidence", func(t *testing.T) {
		t.Parallel()

		got, gotErr := CreateBucket(context.Background(), nil, GCSBucketCreateRequest{})
		if !errors.Is(gotErr, core.ErrObjectStoreContract) || got != (GCSBucketProvisioning{}) {
			t.Fatalf("CreateBucket(zero) = (%v, %v), want zero and errors.Is %v",
				got, gotErr, core.ErrObjectStoreContract)
		}
		client := bucketTestClient(t, http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			writer.WriteHeader(http.StatusCreated)
		}))
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		got, gotErr = CreateBucket(ctx, client, bucketCreateFixture(t, GCSNamespaceFlat))
		if !errors.Is(gotErr, context.Canceled) || got != (GCSBucketProvisioning{}) {
			t.Fatalf("CreateBucket(canceled) = (%v, %v), want zero and errors.Is context.Canceled", got, gotErr)
		}
	})
}

func bucketCreateFixture(t testing.TB, namespace GCSNamespace) GCSBucketCreateRequest {
	t.Helper()

	project, err := ParseGCSProjectID("primitive-project")
	if err != nil {
		t.Fatalf("ParseGCSProjectID() error = %v, want nil", err)
	}
	bucket, err := ParseGCSBucket("primitive-custody")
	if err != nil {
		t.Fatalf("ParseGCSBucket() error = %v, want nil", err)
	}
	location, err := ParseGCSLocation("northamerica-northeast2")
	if err != nil {
		t.Fatalf("ParseGCSLocation() error = %v, want nil", err)
	}
	request := GCSBucketCreateRequest{
		Project: project, Bucket: bucket, Location: location, Namespace: namespace,
	}
	if err := request.Validate(); err != nil {
		t.Fatalf("GCSBucketCreateRequest.Validate() error = %v, want nil", err)
	}
	return request
}

func parsedGCSProject(t testing.TB, value string) GCSProjectID {
	t.Helper()

	project, err := ParseGCSProjectID(value)
	if err != nil {
		t.Fatalf("ParseGCSProjectID(%q) error = %v, want nil", value, err)
	}
	return project
}

func parsedGCSBucket(t testing.TB, value string) GCSBucket {
	t.Helper()

	bucket, err := ParseGCSBucket(value)
	if err != nil {
		t.Fatalf("ParseGCSBucket(%q) error = %v, want nil", value, err)
	}
	return bucket
}

func parsedGCSLocation(t testing.TB, value string) GCSLocation {
	t.Helper()

	location, err := ParseGCSLocation(value)
	if err != nil {
		t.Fatalf("ParseGCSLocation(%q) error = %v, want nil", value, err)
	}
	return location
}

func bucketTestClient(t testing.TB, handler http.Handler) *GCSClient {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	client, err := storage.NewClient(
		context.Background(), option.WithEndpoint(server.URL+"/storage/v1/"),
		option.WithoutAuthentication(), option.WithHTTPClient(server.Client()),
	)
	if err != nil {
		t.Fatalf("storage.NewClient(test provider) error = %v, want nil", err)
	}
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("storage.Client.Close() error = %v, want nil", err)
		}
	})
	return &GCSClient{client: client}
}
