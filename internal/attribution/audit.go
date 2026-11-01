package attribution

import (
	"encoding/json"
	"sort"
	"strings"
	"time"
)

// AuditEvent is one parsed Kubernetes audit-log line (audit.k8s.io/v1,
// one JSON object per line). Only identity and what/where fields are
// kept: at Metadata level there are no request bodies to leak.
type AuditEvent struct {
	User             string   // user.username, e.g. "kubernetes-admin"
	Groups           []string // user.groups
	ImpersonatedUser string   // impersonatedUser.username, "" when absent
	UserAgent        string   // e.g. "kubectl/v1.37.1"
	Verb             string   // get/list/create/update/patch/delete/...
	Resource         string   // objectRef.resource, e.g. "deployments"
	Subresource      string   // objectRef.subresource, "" when absent
	Namespace        string   // objectRef.namespace
	Name             string   // objectRef.name
	APIGroup         string   // objectRef.apiGroup
	Timestamp        time.Time
	Stage            string
}

// ParseAuditLine parses one audit-log JSON line. Unknown fields are
// ignored; missing fields become zero values (never an error for a
// well-formed line). Malformed JSON is an error: a half-parsed identity
// is worse than none.
func ParseAuditLine(line []byte) (AuditEvent, error) {
	var raw struct {
		User struct {
			Username string   `json:"username"`
			Groups   []string `json:"groups"`
		} `json:"user"`
		Impersonated struct {
			Username string `json:"username"`
		} `json:"impersonatedUser"`
		UserAgent string `json:"userAgent"`
		Verb      string `json:"verb"`
		ObjectRef struct {
			Resource    string `json:"resource"`
			Subresource string `json:"subresource"`
			Namespace   string `json:"namespace"`
			Name        string `json:"name"`
			APIGroup    string `json:"apiGroup"`
		} `json:"objectRef"`
		RequestReceived string `json:"requestReceivedTimestamp"`
		StageTimestamp  string `json:"stageTimestamp"`
		Stage           string `json:"stage"`
	}
	dec := json.NewDecoder(strings.NewReader(string(line)))
	if err := dec.Decode(&raw); err != nil {
		return AuditEvent{}, err
	}
	var ts time.Time
	for _, cand := range []string{raw.RequestReceived, raw.StageTimestamp} {
		if cand == "" {
			continue
		}
		if t, err := time.Parse(time.RFC3339Nano, cand); err == nil {
			ts = t
			break
		}
		if t, err := time.Parse(time.RFC3339, cand); err == nil {
			ts = t
			break
		}
	}
	return AuditEvent{
		User:             raw.User.Username,
		Groups:           append([]string(nil), raw.User.Groups...),
		ImpersonatedUser: raw.Impersonated.Username,
		UserAgent:        raw.UserAgent,
		Verb:             raw.Verb,
		Resource:         raw.ObjectRef.Resource,
		Subresource:      raw.ObjectRef.Subresource,
		Namespace:        raw.ObjectRef.Namespace,
		Name:             raw.ObjectRef.Name,
		APIGroup:         raw.ObjectRef.APIGroup,
		Timestamp:        ts,
		Stage:            raw.Stage,
	}, nil
}

// Confidence says how well a change is attributed. Never present a
// client as a person: managedFields names software, not humans.
type Confidence int

const (
	// ConfidenceUnknown: no audit trail, no manager. Say so.
	ConfidenceUnknown Confidence = iota
	// ConfidenceClient: a managedFields manager is known. It names the
	// client (kubectl, a controller, an operator) — not a person.
	ConfidenceClient
	// ConfidencePerson: the audit log names a non-system account.
	ConfidencePerson
)

// String renders the level for logs and receipts.
func (c Confidence) String() string {
	switch c {
	case ConfidencePerson:
		return "person"
	case ConfidenceClient:
		return "client"
	default:
		return "unknown"
	}
}

// Attribution is one verdict about who changed something.
type Attribution struct {
	Level    Confidence
	Identity string // username, or manager/client name; "" when unknown
	When     time.Time
}

// Present decides which identity to show for an audit event. kubectl
// --as impersonation records BOTH: user.username is the credential holder
// who made the request, impersonatedUser.username is who it executed as.
// We present the effective (impersonated) actor, always labeled, and keep
// the requester in the record: showing the impersonated name bare would
// let anyone claim to be anyone with --as.
//
//	("alice@example.com", "impersonated, requested by admin")  --as demo
//	("bob", "direct")                                          normal request
func (e AuditEvent) Present() (identity, qualifier string) {
	if e.ImpersonatedUser != "" {
		return e.ImpersonatedUser, "impersonated, requested by " + e.User
	}
	return e.User, "direct"
}

// isPersonAccount reports whether an audit username identifies a human// operator as opposed to platform machinery. "system:*" accounts
// (controllers, kubelets, service accounts, anonymous) are clients,
// not people. Other names ("kubernetes-admin", "alice") count as
// persons, with a known limit: a shared kubeconfig narrows identity to
// whoever holds that credential, not necessarily a named human.
func isPersonAccount(username string) bool {
	if username == "" {
		return false
	}
	return !strings.HasPrefix(username, "system:")
}

// Resolve combines one audit event with managedFields manager names
// into a single verdict. Precedence: identified person (audit) beats
// identified client (managedFields); both missing means unknown.
// Managers are sorted and the first wins so the result is deterministic.
func Resolve(audit *AuditEvent, managers []string) Attribution {
	if audit != nil && isPersonAccount(audit.User) {
		return Attribution{Level: ConfidencePerson, Identity: audit.User, When: audit.Timestamp}
	}
	if audit != nil && audit.User != "" {
		return Attribution{Level: ConfidenceClient, Identity: audit.User, When: audit.Timestamp}
	}
	ms := append([]string(nil), managers...)
	sort.Strings(ms)
	seen := map[string]bool{}
	uniq := ms[:0]
	for _, m := range ms {
		if m != "" && !seen[m] {
			seen[m] = true
			uniq = append(uniq, m)
		}
	}
	if len(uniq) > 0 {
		return Attribution{Level: ConfidenceClient, Identity: uniq[0]}
	}
	return Attribution{Level: ConfidenceUnknown}
}
