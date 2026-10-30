package attribution

import (
	"encoding/json"
	"sort"
	"testing"
	"time"
)

// Realistic managedFields shapes: kubectl (Update), Argo CD (Apply) and the
// operator itself (Apply), with overlapping ownership on spec.replicas and on
// the container image.
func fixtureObject() map[string]any {
	return map[string]any{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata": map[string]any{
			"name":      "demo",
			"namespace": "demo",
			"managedFields": []any{
				map[string]any{
					"manager":    "kubectl",
					"operation":  "Update",
					"apiVersion": "apps/v1",
					"time":       "2024-05-01T10:00:00Z",
					"fieldsType": "FieldsV1",
					"fieldsV1": map[string]any{
						"f:spec": map[string]any{
							"f:replicas": map[string]any{},
							"f:template": map[string]any{
								"f:spec": map[string]any{
									"f:containers": map[string]any{
										`k:{"name":"web"}`: map[string]any{
											".":       map[string]any{},
											"f:image": map[string]any{},
											"f:name":  map[string]any{},
										},
									},
								},
							},
						},
					},
				},
				map[string]any{
					"manager":    "argocd-application-controller",
					"operation":  "Apply",
					"apiVersion": "apps/v1",
					"time":       "2024-05-02T10:00:00Z",
					"fieldsType": "FieldsV1",
					"fieldsV1": map[string]any{
						"f:spec": map[string]any{
							"f:template": map[string]any{
								"f:spec": map[string]any{
									"f:containers": map[string]any{
										`k:{"name":"web"}`: map[string]any{
											".":           map[string]any{},
											"f:image":     map[string]any{},
											"f:resources": map[string]any{},
										},
									},
								},
							},
						},
					},
				},
				map[string]any{
					"manager":    "umbra-operator",
					"operation":  "Apply",
					"apiVersion": "apps/v1",
					"time":       "2024-05-03T10:00:00Z",
					"fieldsType": "FieldsV1",
					"fieldsV1": map[string]any{
						"f:spec": map[string]any{
							"f:replicas": map[string]any{},
						},
					},
				},
			},
		},
	}
}

func byField(as []FieldAttribution) map[string]FieldAttribution {
	m := make(map[string]FieldAttribution, len(as))
	for _, a := range as {
		m[a.Field] = a
	}
	return m
}

func TestLastWriterWins(t *testing.T) {
	got := byField(Attribute(fixtureObject()))
	want := map[string]string{
		"spec.replicas":                           "umbra-operator",
		"spec.template.spec.containers.image":     "argocd-application-controller",
		"spec.template.spec.containers.name":      "kubectl",
		"spec.template.spec.containers.resources": "argocd-application-controller",
	}
	if len(got) != len(want) {
		t.Fatalf("got %d fields %v, want %d", len(got), got, len(want))
	}
	for f, mgr := range want {
		a, ok := got[f]
		if !ok {
			t.Errorf("missing field %q", f)
			continue
		}
		if a.Manager != mgr {
			t.Errorf("field %q manager = %q, want %q", f, a.Manager, mgr)
		}
		if a.UpdatedAt.IsZero() {
			t.Errorf("field %q has zero time", f)
		}
	}
	if got["spec.replicas"].UpdatedAt.Format(time.RFC3339) != "2024-05-03T10:00:00Z" {
		t.Errorf("spec.replicas time = %v, want 2024-05-03", got["spec.replicas"].UpdatedAt)
	}
}

func TestStableOrder(t *testing.T) {
	got := Attribute(fixtureObject())
	if !sort.SliceIsSorted(got, func(i, j int) bool { return got[i].Field < got[j].Field }) {
		t.Fatalf("not sorted by field: %v", got)
	}
}

func TestIgnoresStatusSubresource(t *testing.T) {
	obj := fixtureObject()
	meta := obj["metadata"].(map[string]any)
	mf := meta["managedFields"].([]any)
	mf = append(mf, map[string]any{
		"manager":     "kube-controller-manager",
		"operation":   "Update",
		"apiVersion":  "apps/v1",
		"time":        "2024-05-04T10:00:00Z", // newer, must not win
		"fieldsType":  "FieldsV1",
		"subresource": "status",
		"fieldsV1": map[string]any{
			"f:spec": map[string]any{"f:replicas": map[string]any{}},
		},
	})
	meta["managedFields"] = mf
	got := byField(Attribute(obj))
	if got["spec.replicas"].Manager != "umbra-operator" {
		t.Fatalf("status entry won: %v", got["spec.replicas"])
	}
}

func TestIgnoresOtherOperations(t *testing.T) {
	obj := fixtureObject()
	meta := obj["metadata"].(map[string]any)
	mf := meta["managedFields"].([]any)
	mf = append(mf, map[string]any{
		"manager":    "someone",
		"operation":  "Delete",
		"time":       "2024-06-01T10:00:00Z",
		"fieldsType": "FieldsV1",
		"fieldsV1": map[string]any{
			"f:spec": map[string]any{"f:replicas": map[string]any{}},
		},
	})
	meta["managedFields"] = mf
	if got := byField(Attribute(obj)); got["spec.replicas"].Manager != "umbra-operator" {
		t.Fatalf("unknown operation won: %v", got["spec.replicas"])
	}
}

func TestEmptyObjects(t *testing.T) {
	for name, obj := range map[string]map[string]any{
		"nil":          nil,
		"empty":        {},
		"no metadata":  {"kind": "Deployment"},
		"no fields":    {"metadata": map[string]any{"name": "x"}},
		"empty list":   {"metadata": map[string]any{"managedFields": []any{}}},
		"bad shape":    {"metadata": map[string]any{"managedFields": "nope"}},
		"empty fields": {"metadata": map[string]any{"managedFields": []any{map[string]any{"manager": "x", "operation": "Update"}}}},
	} {
		if got := Attribute(obj); len(got) != 0 {
			t.Errorf("%s: got %v, want empty", name, got)
		}
	}
}

// The package operates on decoded YAML/JSON: prove a JSON round-trip keeps
// the same attribution.
func TestJSONDecodedObject(t *testing.T) {
	raw, err := json.Marshal(fixtureObject())
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	got := byField(Attribute(decoded))
	if got["spec.replicas"].Manager != "umbra-operator" {
		t.Fatalf("json decoded: %v", got["spec.replicas"])
	}
}
