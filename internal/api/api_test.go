package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/Jamshaa/Umbra/api/v1alpha1"
)

func apiScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	if err := v1alpha1.AddToScheme(s); err != nil {
		t.Fatal(err)
	}
	return s
}

func seedFinding(name, state, liveYaml, patch string) *v1alpha1.Finding {
	return &v1alpha1.Finding{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: v1alpha1.FindingSpec{
			Severity:         v1alpha1.FindingSeverityHigh,
			Category:         v1alpha1.FindingCategoryResources,
			TargetRef:        v1alpha1.TargetRef{Kind: "Deployment", Namespace: "demo", Name: "web"},
			Detector:         "scanner",
			Diagnosis:        "no resources",
			ProposedPatch:    patch,
			PrePatchManifest: liveYaml,
			State:            v1alpha1.FindingState(state),
		},
	}
}

func get(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestFindingsEmpty(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(apiScheme(t)).Build()
	rec := get(t, NewHandler(c, ""), "/api/findings")
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d", rec.Code)
	}
	var views []FindingView
	if err := json.Unmarshal(rec.Body.Bytes(), &views); err != nil {
		t.Fatal(err)
	}
	if len(views) != 0 {
		t.Fatalf("views = %+v, want empty", views)
	}
}

func TestFindingsMappedAndSanitized(t *testing.T) {
	live := "apiVersion: apps/v1\npassword=hunter2-hunter2\n"
	c := fake.NewClientBuilder().WithScheme(apiScheme(t)).WithObjects(
		seedFinding("f01", "Verified", live, "resources:\n  limits: {}"),
		seedFinding("f03", "FixProposed", "manifest: 1", ""),
	).Build()
	rec := get(t, NewHandler(c, ""), "/api/findings")
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d", rec.Code)
	}
	var views []FindingView
	if err := json.Unmarshal(rec.Body.Bytes(), &views); err != nil {
		t.Fatal(err)
	}
	if len(views) != 2 {
		t.Fatalf("len = %d, want 2", len(views))
	}
	byID := map[string]FindingView{}
	for _, v := range views {
		byID[v.ID] = v
	}
	f01 := byID["f01"]
	if f01.Severity != "high" || f01.State != "verified" || f01.Kind != "Deployment" || f01.Target != "demo/web" {
		t.Fatalf("f01 mapped wrong: %+v", f01)
	}
	for _, leak := range []string{"hunter2"} {
		if strings.Contains(f01.LiveYaml, leak) {
			t.Fatalf("secret survived in liveYaml: %q", f01.LiveYaml)
		}
	}
	if len(f01.DiffAdded) != 2 || len(f01.DiffRemoved) != 0 {
		t.Fatalf("diff split wrong: %+v", f01)
	}
	if got := byID["f03"].State; got != "fixproposed" {
		t.Fatalf("f03 state = %q", got)
	}
}

func TestMethodNotAllowed(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(apiScheme(t)).Build()
	req := httptest.NewRequest(http.MethodPost, "/api/findings", nil)
	rec := httptest.NewRecorder()
	NewHandler(c, "").ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("code = %d, want 405", rec.Code)
	}
}

func TestSecurityHeaders(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(apiScheme(t)).Build()
	rec := get(t, NewHandler(c, ""), "/api/findings")
	for k, want := range map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"Referrer-Policy":        "no-referrer",
		"Cache-Control":          "no-store",
	} {
		if got := rec.Header().Get(k); got != want {
			t.Fatalf("header %s = %q, want %q", k, got, want)
		}
	}
}

func TestNoDirectoryListing(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(apiScheme(t)).Build()
	dir := t.TempDir()
	sub := filepath.Join(dir, "assets")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "app.js"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	h := NewHandler(c, dir)
	// Exact files still serve.
	if rec := get(t, h, "/assets/app.js"); rec.Code != http.StatusOK {
		t.Fatalf("asset code = %d, want 200", rec.Code)
	}
	// Directory URLs without an index must not list.
	if rec := get(t, h, "/assets/"); rec.Code != http.StatusNotFound {
		t.Fatalf("listing code = %d, want 404", rec.Code)
	}
}

func TestDistFallback(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(apiScheme(t)).Build()
	rec := get(t, NewHandler(c, ""), "/")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("code = %d, want 503", rec.Code)
	}
	rec = get(t, NewHandler(c, filepath.Join(t.TempDir(), "nope")), "/")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("code = %d, want 404 for missing dir", rec.Code)
	}
}
