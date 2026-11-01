package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Jamshaa/Umbra/internal/evidence"
)

// maxTailLines caps get_logs output: prompts stay small and bounded.
const maxTailLines = 500

// defaultTailLines is used when the model omits tail.
const defaultTailLines = 100

// ToolDef is one function tool offered to the model (flat Responses API
// shape). Parameters holds the JSON Schema the model must follow;
// arguments are always re-validated in code (never trusted).
type ToolDef struct {
	Name        string
	Description string
	Parameters  string
}

// Tools is the full read-only set plus propose_patch. The model gets no
// shell, no free kubectl and no credentials.
func Tools() []ToolDef {
	obj := func(props string, required ...string) string {
		req := ""
		if len(required) > 0 {
			q := make([]string, 0, len(required))
			for _, r := range required {
				q = append(q, `"`+r+`"`)
			}
			req = `,"required":[` + strings.Join(q, ",") + `]`
		}
		return `{"type":"object","properties":{` + props + `}` + req + `}`
	}
	str := func(desc string) string { return `{"type":"string","description":"` + desc + `"}` }
	intg := func(desc string) string { return `{"type":"integer","description":"` + desc + `"}` }
	return []ToolDef{
		{
			Name:        "get_manifest",
			Description: "Read one workload manifest (sanitized). No live state.",
			Parameters: obj(
				`"kind":`+str("Kind, e.g. Deployment")+
					`,"name":`+str("Object name")+
					`,"namespace":`+str("Namespace"),
				"kind", "name", "namespace"),
		},
		{
			Name:        "get_events",
			Description: "Recent warning/normal events for one object, newest last (max 20, sanitized).",
			Parameters: obj(
				`"kind":`+str("Kind, e.g. Pod")+
					`,"name":`+str("Object name")+
					`,"namespace":`+str("Namespace"),
				"kind", "name", "namespace"),
		},
		{
			Name:        "get_pod_status",
			Description: "Phase, readiness, restarts and last termination reason of one pod.",
			Parameters: obj(
				`"namespace":`+str("Namespace")+
					`,"name":`+str("Pod name"),
				"namespace", "name"),
		},
		{
			Name:        "get_logs",
			Description: "Last lines of one container log (sanitized). Tail capped.",
			Parameters: obj(
				`"namespace":`+str("Namespace")+
					`,"pod":`+str("Pod name")+
					`,"container":`+str("Container name, empty for the first one")+
					`,"tail":`+intg("Max lines, 1-500, default 100"),
				"namespace", "pod"),
		},
		{
			Name:        "get_related",
			Description: "Dependency neighbourhood: selecting Services, referenced ConfigMaps/Secrets/PVCs, service account and its RoleBindings (names only, never secret values).",
			Parameters: obj(
				`"kind":`+str("Kind, e.g. Deployment")+
					`,"name":`+str("Object name")+
					`,"namespace":`+str("Namespace"),
				"kind", "name", "namespace"),
		},
		{
			Name:        "propose_patch",
			Description: "Propose (never apply) a fix as ops. Smallest patch that fixes the root cause; never widen RBAC, touch secret values, change namespaces or immutable fields; when unsure set needs_human.",
			Parameters:  `{"type":"object","properties":{"finding_id":{"type":"string"},"root_cause":{"type":"string","maxLength":600},"ops":{"type":"array","maxItems":12,"items":{"type":"object","properties":{"op":{"enum":["add","replace","remove"]},"path":{"type":"string"},"value":{}},"required":["op","path"]}},"confidence":{"type":"number","minimum":0,"maximum":1},"needs_human":{"type":"boolean"},"rationale":{"type":"string","maxLength":800}},"required":["finding_id","root_cause","ops","confidence","needs_human"]}`,
		},
	}
}

// validatedArgs is the checked form of one tool call.
type validatedArgs struct {
	kind, name, namespace string
	pod, container        string
	tail                  int
}

// needStr extracts a required non-empty string field.
func needStr(args map[string]any, field string) (string, error) {
	v, ok := args[field].(string)
	if !ok || strings.TrimSpace(v) == "" {
		return "", fmt.Errorf("missing required string %q", field)
	}
	return v, nil
}

// validateReadArgs checks read-tool args. get_pod_status and get_logs
// address the pod as "pod"; the rest address kind/name.
func validateReadArgs(name string, args map[string]any) (validatedArgs, error) {
	var v validatedArgs
	var err error
	if name == "get_pod_status" || name == "get_logs" {
		if v.name, err = needStr(args, "pod"); err != nil {
			return validatedArgs{}, err
		}
	} else {
		if v.kind, err = needStr(args, "kind"); err != nil {
			return validatedArgs{}, err
		}
		if v.name, err = needStr(args, "name"); err != nil {
			return validatedArgs{}, err
		}
	}
	if v.namespace, err = needStr(args, "namespace"); err != nil {
		return validatedArgs{}, err
	}
	if name == "get_logs" {
		if c, ok := args["container"].(string); ok {
			v.container = c
		}
		v.tail = defaultTailLines
		if t, ok := args["tail"]; ok && t != nil {
			f, ok := t.(float64)
			if !ok || f != float64(int(f)) || f < 1 {
				return validatedArgs{}, fmt.Errorf("tail must be an integer >= 1")
			}
			v.tail = int(f)
			if v.tail > maxTailLines {
				v.tail = maxTailLines
			}
		}
	}
	return v, nil
}

// Op is one validated patch operation.
type Op struct {
	Op    string
	Path  string
	Value any
	HasV  bool
}

// Proposal is a validated propose_patch call: ready for the harness.
// Value stays decoded JSON (never re-serialized blindly).
type Proposal struct {
	FindingID   string
	RootCause   string
	Ops         []Op
	Confidence  float64
	NeedsHuman  bool
	Rationale   string
	RawCallID   string
	RawArgument string
}

func runeLen(s string) int { return len([]rune(s)) }

// validateProposal enforces the propose_patch schema in code. Unknown
// fields are ignored (forward-compatible); every known constraint is
// checked, including bounds the schema states.
func validateProposal(args map[string]any) (Proposal, error) {
	var p Proposal
	var err error
	if p.FindingID, err = needStr(args, "finding_id"); err != nil {
		return p, err
	}
	rc, ok := args["root_cause"].(string)
	if !ok || strings.TrimSpace(rc) == "" {
		return p, fmt.Errorf("missing required string %q", "root_cause")
	}
	if runeLen(rc) > 600 {
		return p, fmt.Errorf("root_cause exceeds 600 chars")
	}
	p.RootCause = rc
	rawOps, ok := args["ops"].([]any)
	if !ok || len(rawOps) == 0 {
		return p, fmt.Errorf("ops must be a non-empty array")
	}
	if len(rawOps) > 12 {
		return p, fmt.Errorf("ops exceeds 12 items")
	}
	for i, ro := range rawOps {
		om, ok := ro.(map[string]any)
		if !ok {
			return p, fmt.Errorf("ops[%d] must be an object", i)
		}
		op, _ := om["op"].(string)
		if op != "add" && op != "replace" && op != "remove" {
			return p, fmt.Errorf("ops[%d].op must be add|replace|remove", i)
		}
		path, _ := om["path"].(string)
		if strings.TrimSpace(path) == "" {
			return p, fmt.Errorf("ops[%d].path is required", i)
		}
		v, hasV := om["value"]
		if (op == "add" || op == "replace") && !hasV {
			return p, fmt.Errorf("ops[%d]: %s needs a value", i, op)
		}
		p.Ops = append(p.Ops, Op{Op: op, Path: path, Value: v, HasV: hasV})
	}
	cf, ok := args["confidence"].(float64)
	if !ok || cf < 0 || cf > 1 {
		return p, fmt.Errorf("confidence must be a number in [0,1]")
	}
	p.Confidence = cf
	nh, ok := args["needs_human"].(bool)
	if !ok {
		return p, fmt.Errorf("needs_human must be a boolean")
	}
	p.NeedsHuman = nh
	if r, ok := args["rationale"].(string); ok {
		if runeLen(r) > 800 {
			return p, fmt.Errorf("rationale exceeds 800 chars")
		}
		p.Rationale = r
	} else if _, present := args["rationale"]; present {
		return p, fmt.Errorf("rationale must be a string")
	}
	return p, nil
}

// dispatch runs one validated call. Read tools hit Kube and return
// sanitized text; propose_patch validates and returns the proposal
// without applying anything, ever.
func dispatch(ctx context.Context, k Kube, name string, args map[string]any) (string, *Proposal, error) {
	if name == "propose_patch" {
		p, err := validateProposal(args)
		if err != nil {
			return "", nil, err
		}
		out, _ := json.Marshal(map[string]any{
			"finding_id":  p.FindingID,
			"ops":         len(p.Ops),
			"confidence":  p.Confidence,
			"needs_human": p.NeedsHuman,
		})
		return "proposal recorded: " + string(out), &p, nil
	}
	v, err := validateReadArgs(name, args)
	if err != nil {
		return "", nil, err
	}
	switch name {
	case "get_manifest":
		obj, err := k.Get(ctx, v.kind, v.name, v.namespace)
		if err != nil {
			return "", nil, err
		}
		raw, _ := json.Marshal(obj)
		return evidence.Sanitize(string(raw)), nil, nil
	case "get_events":
		evs, err := EventsFor(ctx, k, v.kind, v.name, v.namespace)
		if err != nil {
			return "", nil, err
		}
		if len(evs) == 0 {
			return "no events", nil, nil
		}
		return evidence.Sanitize(strings.Join(evs, "\n")), nil, nil
	case "get_pod_status":
		obj, err := k.Get(ctx, "Pod", v.name, v.namespace)
		if err != nil {
			return "", nil, err
		}
		st, _ := obj["status"].(map[string]any)
		phase, _ := st["phase"].(string)
		ready := false
		var restarts int
		var term string
		if cs, _ := st["containerStatuses"].([]any); len(cs) > 0 {
			if cm, _ := cs[0].(map[string]any); cm != nil {
				ready, _ = cm["ready"].(bool)
				if r, ok := cm["restartCount"].(float64); ok {
					restarts = int(r)
				}
				if ls, _ := cm["lastState"].(map[string]any); ls != nil {
					if tm, _ := ls["terminated"].(map[string]any); tm != nil {
						term, _ = tm["reason"].(string)
					}
				}
			}
		}
		return evidence.Sanitize(fmt.Sprintf("phase=%s ready=%v restarts=%d lastTermination=%s", phase, ready, restarts, term)), nil, nil
	case "get_logs":
		out, err := k.Logs(ctx, v.namespace, v.name, v.container, v.tail)
		if err != nil {
			return "", nil, err
		}
		lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
		if len(lines) > maxTailLines {
			lines = lines[len(lines)-maxTailLines:]
		}
		return evidence.Sanitize(strings.Join(lines, "\n")), nil, nil
	case "get_related":
		rel, err := GetRelated(ctx, k, v.kind, v.name, v.namespace)
		if err != nil {
			return "", nil, err
		}
		raw, _ := json.Marshal(rel)
		return string(raw), nil, nil
	}
	return "", nil, fmt.Errorf("unknown tool %q", name)
}
