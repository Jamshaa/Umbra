package attribution

import (
	"sort"
	"strings"
	"time"
)

// FieldAttribution says which manager last wrote a field and when.
type FieldAttribution struct {
	Field     string
	Manager   string
	UpdatedAt time.Time
}

// Attribute returns one entry per field owned in metadata.managedFields,
// keeping the last writer per field (latest time; later list entry wins ties)
// and sorting by field. Only Update/Apply operations count; entries for the
// status subresource are ignored. A missing or malformed managedFields yields
// an empty (nil) slice, never an error.
func Attribute(obj map[string]any) []FieldAttribution {
	best := make(map[string]FieldAttribution)
	for _, e := range managedEntries(obj) {
		for _, f := range fieldsOf(e.fields) {
			cur, ok := best[f]
			if !ok || !e.when.Before(cur.UpdatedAt) {
				best[f] = FieldAttribution{Field: f, Manager: e.manager, UpdatedAt: e.when}
			}
		}
	}
	if len(best) == 0 {
		return nil
	}
	out := make([]FieldAttribution, 0, len(best))
	for _, a := range best {
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Field < out[j].Field })
	return out
}

type managedEntry struct {
	manager string
	when    time.Time
	fields  map[string]any
}

func managedEntries(obj map[string]any) []managedEntry {
	meta, _ := obj["metadata"].(map[string]any)
	if meta == nil {
		return nil
	}
	var raw []any
	switch v := meta["managedFields"].(type) {
	case []any:
		raw = v
	case []map[string]any:
		for _, m := range v {
			raw = append(raw, m)
		}
	default:
		return nil
	}
	var out []managedEntry
	for _, item := range raw {
		m, _ := item.(map[string]any)
		if m == nil {
			continue
		}
		if op, _ := m["operation"].(string); op != "Update" && op != "Apply" {
			continue
		}
		if sub, _ := m["subresource"].(string); sub == "status" {
			continue
		}
		manager, _ := m["manager"].(string)
		if manager == "" {
			continue
		}
		fields, _ := m["fieldsV1"].(map[string]any)
		if len(fields) == 0 {
			continue
		}
		out = append(out, managedEntry{manager: manager, when: parseTime(m["time"]), fields: fields})
	}
	return out
}

func parseTime(v any) time.Time {
	switch t := v.(type) {
	case time.Time:
		return t
	case string:
		if tm, err := time.Parse(time.RFC3339Nano, t); err == nil {
			return tm
		}
		if tm, err := time.Parse(time.RFC3339, t); err == nil {
			return tm
		}
	}
	return time.Time{}
}

// fieldsOf flattens a fieldsV1 tree into dotted paths ("spec.replicas").
// f:-prefixed keys add a segment; list-item keys (k:, v:) and "." are
// transparent and merge into the parent path.
// shortcut: per-item list identity is merged (containers[name=x] and
// containers[name=y] both report as containers.<field>); split them if T-27
// needs per-item diffs.
func fieldsOf(node map[string]any) []string {
	seen := make(map[string]struct{})
	var walk func(n map[string]any, prefix string)
	walk = func(n map[string]any, prefix string) {
		for k, v := range n {
			switch {
			case strings.HasPrefix(k, "f:"):
				cur := k[2:]
				if prefix != "" {
					cur = prefix + "." + cur
				}
				m, _ := v.(map[string]any)
				if len(m) == 0 {
					seen[cur] = struct{}{}
					continue
				}
				before := len(seen)
				walk(m, cur)
				if len(seen) == before {
					seen[cur] = struct{}{}
				}
			case k == "." || strings.HasPrefix(k, "k:") || strings.HasPrefix(k, "v:"):
				m, _ := v.(map[string]any)
				if len(m) == 0 {
					continue
				}
				walk(m, prefix)
			default:
				cur := k
				if prefix != "" {
					cur = prefix + "." + cur
				}
				m, _ := v.(map[string]any)
				if len(m) == 0 {
					seen[cur] = struct{}{}
					continue
				}
				before := len(seen)
				walk(m, cur)
				if len(seen) == before {
					seen[cur] = struct{}{}
				}
			}
		}
	}
	walk(node, "")
	out := make([]string, 0, len(seen))
	for f := range seen {
		out = append(out, f)
	}
	return out
}
