package invariants

import (
	"fmt"
	"strconv"
	"strings"

	sigsyaml "sigs.k8s.io/yaml"
)

// Op is one validated patch operation on a manifest document.
// Path uses JSON-pointer segments ("/a/b/0/c"); "~0" and "~1" escapes
// apply. Only maps built by decodeOne are supported (string keys).
type Op struct {
	Op    string
	Path  string
	Value any
	HasV  bool
}

// ApplyOps returns base with ops applied, without mutating base.
// add creates missing intermediate maps; "-" appends to arrays;
// remove deletes keys or array items. Anything else is an error.
func ApplyOps(base []byte, ops []Op) ([]byte, error) {
	doc, err := decodeOne(base)
	if err != nil {
		return nil, err
	}
	for i, op := range ops {
		if err := applyOne(doc, op); err != nil {
			return nil, fmt.Errorf("op %d (%s %s): %w", i, op.Op, op.Path, err)
		}
	}
	out, err := marshalDoc(doc)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// marshalDoc renders back to block YAML (what kubectl applies).
func marshalDoc(doc map[string]any) ([]byte, error) {
	out, err := sigsyaml.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("invariants: marshal: %w", err)
	}
	return out, nil
}

func unescape(seg string) string {
	seg = strings.ReplaceAll(seg, "~1", "/")
	return strings.ReplaceAll(seg, "~0", "~")
}

func splitOpPath(p string) ([]string, error) {
	if p == "" || p[0] != '/' {
		return nil, fmt.Errorf("path must start with /: %q", p)
	}
	if p == "/" {
		return nil, fmt.Errorf("empty path")
	}
	raw := strings.Split(p[1:], "/")
	out := make([]string, 0, len(raw))
	for _, s := range raw {
		out = append(out, unescape(s))
	}
	return out, nil
}

func applyOne(doc map[string]any, op Op) error {
	segs, err := splitOpPath(op.Path)
	if err != nil {
		return err
	}
	// applyAt mutates nested maps in place and returns replaced slices
	// upward; the root map itself is always the same object.
	_, err = applyAt(doc, segs, op, op.Op == "add")
	return err
}

// applyAt returns the updated value for cur (slices may be replaced).
func applyAt(cur any, segs []string, op Op, create bool) (any, error) {
	if len(segs) == 0 {
		return nil, fmt.Errorf("empty path")
	}
	s, last := segs[0], len(segs) == 1
	if s == "-" && !last {
		return nil, fmt.Errorf(`"-" appends only at the end of a path`)
	}
	switch node := cur.(type) {
	case map[string]any:
		if last {
			return applyLeafMap(node, s, op)
		}
		child, ok := node[s]
		if !ok {
			if !create {
				return nil, fmt.Errorf("missing key %q", s)
			}
			child = map[string]any{}
			node[s] = child
		}
		nv, err := applyAt(child, segs[1:], op, create)
		if err != nil {
			return nil, err
		}
		node[s] = nv
		return node, nil
	case []any:
		if last && s == "-" {
			return applyAppend(node, op)
		}
		idx, derr := strconv.Atoi(s)
		if derr != nil || idx < 0 || idx >= len(node) {
			return nil, fmt.Errorf("bad array index %q", s)
		}
		if last {
			return applyLeafList(node, idx, op)
		}
		nv, err := applyAt(node[idx], segs[1:], op, create)
		if err != nil {
			return nil, err
		}
		node[idx] = nv
		return node, nil
	default:
		return nil, fmt.Errorf("cannot descend into %T at %q", cur, s)
	}
}

func applyLeafMap(node map[string]any, key string, op Op) (any, error) {
	switch op.Op {
	case "add", "replace":
		if op.Op == "replace" {
			if _, ok := node[key]; !ok {
				return nil, fmt.Errorf("replace target missing: %q", key)
			}
		}
		if !op.HasV {
			return nil, fmt.Errorf("add/replace needs a value")
		}
		node[key] = op.Value
		return node, nil
	case "remove":
		if _, ok := node[key]; !ok {
			return nil, fmt.Errorf("remove target missing: %q", key)
		}
		delete(node, key)
		return node, nil
	}
	return nil, fmt.Errorf("unknown op %q", op.Op)
}

func applyLeafList(node []any, idx int, op Op) (any, error) {
	switch op.Op {
	case "add":
		if !op.HasV {
			return nil, fmt.Errorf("add needs a value")
		}
		// JSON Patch add inserts before idx; "-" appends (handled below).
		out := make([]any, 0, len(node)+1)
		out = append(out, node[:idx]...)
		out = append(out, op.Value)
		out = append(out, node[idx:]...)
		return out, nil
	case "replace":
		if !op.HasV {
			return nil, fmt.Errorf("replace needs a value")
		}
		node[idx] = op.Value
		return node, nil
	case "remove":
		return append(node[:idx], node[idx+1:]...), nil
	}
	return nil, fmt.Errorf("unknown op %q", op.Op)
}

// applyAppend handles a trailing "-" (append to array).
func applyAppend(node []any, op Op) (any, error) {
	if op.Op != "add" {
		return nil, fmt.Errorf("only add supports append")
	}
	if !op.HasV {
		return nil, fmt.Errorf("add needs a value")
	}
	return append(node, op.Value), nil
}
