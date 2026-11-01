package invariants

import (
	"strconv"
	"strings"
	"testing"
)

func apply(t *testing.T, base string, ops []Op) map[string]any {
	t.Helper()
	out, err := ApplyOps([]byte(base), ops)
	if err != nil {
		t.Fatal(err)
	}
	d, err := decodeOne(out)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func get(t *testing.T, doc map[string]any, path string) any {
	t.Helper()
	segs := strings.Split(strings.TrimPrefix(path, "/"), "/")
	var cur any = doc
	for _, s := range segs {
		s = strings.ReplaceAll(strings.ReplaceAll(s, "~1", "/"), "~0", "~")
		switch node := cur.(type) {
		case map[string]any:
			v, ok := node[s]
			if !ok {
				t.Fatalf("missing %q", s)
			}
			cur = v
		case []any:
			n, err := strconv.Atoi(s)
			if err != nil || n < 0 || n >= len(node) {
				t.Fatalf("bad index %q", s)
			}
			cur = node[n]
		default:
			t.Fatalf("not addressable at %q", s)
		}
	}
	return cur
}

func TestApplyNestedCreatesParents(t *testing.T) {
	doc := apply(t, `{"kind":"Deployment"}`, []Op{
		{Op: "add", Path: "/spec/template/spec/containers", Value: []any{}, HasV: true},
		{Op: "add", Path: "/a/b/c", Value: float64(1), HasV: true},
	})
	if got := get(t, doc, "/a/b/c"); got != float64(1) {
		t.Fatalf("nested add failed: %v", got)
	}
}

func TestApplyArrayForms(t *testing.T) {
	base := `{"list":[{"n":"a"},{"n":"b"}]}`
	doc := apply(t, base, []Op{{Op: "replace", Path: "/list/1/n", Value: "B", HasV: true}})
	if got := get(t, doc, "/list/1/n"); got != "B" {
		t.Fatalf("replace failed: %v", doc)
	}
	doc = apply(t, base, []Op{{Op: "add", Path: "/list/-", Value: map[string]any{"n": "c"}, HasV: true}})
	if l := get(t, doc, "/list"); len(l.([]any)) != 3 {
		t.Fatalf("append failed: %v", doc)
	}
	doc = apply(t, base, []Op{{Op: "add", Path: "/list/0", Value: map[string]any{"n": "z"}, HasV: true}})
	if got := get(t, doc, "/list/0/n"); got != "z" {
		t.Fatalf("insert failed: %v", doc)
	}
	doc = apply(t, base, []Op{{Op: "remove", Path: "/list/0", HasV: false}})
	l := get(t, doc, "/list").([]any)
	if len(l) != 1 || l[0].(map[string]any)["n"] != "b" {
		t.Fatalf("remove shifted wrong: %v", doc)
	}
}

func TestApplyEscapes(t *testing.T) {
	doc := apply(t, `{"a/b":1}`, []Op{{Op: "replace", Path: "/a~1b", Value: float64(2), HasV: true}})
	if got := get(t, doc, "/a~1b"); got != float64(2) {
		t.Fatalf("escaped key not addressed: %v", doc)
	}
}

func TestApplyErrors(t *testing.T) {
	base := `{"a":{"b":[1]}}`
	for _, tc := range []struct {
		name string
		ops  []Op
	}{
		{"no leading slash", []Op{{Op: "add", Path: "a", Value: 1, HasV: true}}},
		{"unknown op", []Op{{Op: "move", Path: "/a", HasV: false}}},
		{"replace missing", []Op{{Op: "replace", Path: "/zz", Value: 1, HasV: true}}},
		{"remove missing", []Op{{Op: "remove", Path: "/zz"}}},
		{"add no value", []Op{{Op: "add", Path: "/zz"}}},
		{"bad index", []Op{{Op: "add", Path: "/a/b/9", Value: 1, HasV: true}}},
		{"descend scalar", []Op{{Op: "add", Path: "/a/b/c", Value: 1, HasV: true}}},
		{"append non-tail", []Op{{Op: "remove", Path: "/-/x", HasV: false}}},
	} {
		if _, err := ApplyOps([]byte(base), tc.ops); err == nil {
			t.Fatalf("%s: expected error", tc.name)
		}
	}
}
