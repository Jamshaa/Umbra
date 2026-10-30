// Package evidence guards everything persisted to Finding status.
//
// Every string that reaches status (timeline tool args, observations,
// diagnostics) passes through Sanitize first, so credentials or private
// host context never land in etcd or in API responses. Anything that
// looks like a secret is replaced with the placeholder Redacted
// ("<REPLACE-ME>", per SECURITY.md §4.d). Structural YAML (manifest
// fields, resource specs) passes through byte-identical.
package evidence

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
)

// Redacted is the only placeholder this package ever emits.
const Redacted = "<REPLACE-ME>"

// secretWords are matched as whole words against key tokens (split on
// separators and camelCase boundaries), so structural keys such as
// "matchLabels" or "containerPort" never match.
var secretWords = map[string]bool{
	"password": true, "passwd": true, "pwd": true,
	"secret": true, "secrets": true,
	"token": true, "tokens": true,
	"key": true, "keys": true, "apikey": true,
	"auth": true, "authorization": true, "bearer": true,
	"credential": true, "credentials": true,
	"privatekey": true, "passphrase": true,
	"kubeconfig": true,
}

// LooksLikeKubeconfig reports whether s carries a kubeconfig document
// (YAML or JSON). It only looks at structural markers, never at values.
func LooksLikeKubeconfig(s string) bool {
	hasKind := strings.Contains(s, "kind: Config") ||
		strings.Contains(s, `"kind": "Config"`) ||
		strings.Contains(s, `"kind":"Config"`)
	return hasKind && strings.Contains(s, "clusters") && strings.Contains(s, "users")
}

// Fingerprint returns the lowercase hex sha256 of s. It is the audit
// hash recorded on TimelineEntry for the sanitized args+result.
func Fingerprint(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

var (
	// key=value or key: value with the value on the same line (quoted or bare).
	kvRe = regexp.MustCompile(`([A-Za-z0-9_.-]+)[ \t]*([:=])[ \t]*("[^"\n]*"|'[^'\n]*'|[^\s,;][^\s]*)`)

	awsKeyRe    = regexp.MustCompile(`\b(AKIA|ASIA)[0-9A-Z]{16}\b`)
	ghTokenRe   = regexp.MustCompile(`\b(ghp|gho|ghu|ghs|ghr)_[A-Za-z0-9]{20,}\b`)
	ghPatRe     = regexp.MustCompile(`\bgithub_pat_[A-Za-z0-9_]{20,}\b`)
	slackRe     = regexp.MustCompile(`\bxox[bpars]-[A-Za-z0-9-]{10,}\b`)
	apiKeyRe    = regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{20,}\b`)
	jwtRe       = regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{8,}\b`)
	bearerRe    = regexp.MustCompile(`(?i)\bBearer\s+[A-Za-z0-9._~+/=-]{10,}`)
	privKeyRe   = regexp.MustCompile(`(?s)-----BEGIN [A-Z0-9 ]*PRIVATE KEY-----.*?-----END [A-Z0-9 ]*PRIVATE KEY-----`)
	urlAuthRe   = regexp.MustCompile(`(https?://)[^/\s:@]+:[^/\s@]+@`)
	ipv4PrivRe  = regexp.MustCompile(`\b(10\.\d{1,3}\.\d{1,3}\.\d{1,3}|172\.(1[6-9]|2\d|3[01])\.\d{1,3}\.\d{1,3}|192\.168\.\d{1,3}\.\d{1,3})\b`)
	localHostRe = regexp.MustCompile(`\b[A-Za-z0-9]([A-Za-z0-9.-]*[A-Za-z0-9])?\.(local|internal|lan)\b`)
)

// isSecretKey reports whether a key token names a credential, comparing
// whole words so "monkey" never matches "key" and "matchLabels" never
// matches "secret".
func isSecretKey(token string) bool {
	// Split on separators first.
	for _, chunk := range strings.FieldsFunc(token, func(r rune) bool {
		return r == '_' || r == '-' || r == '.' || r == '/' || r == ' '
	}) {
		// Then on lowerCamel boundaries: "apiKey" -> "api Key".
		var words []string
		start := 0
		rs := []rune(chunk)
		for i := 1; i < len(rs); i++ {
			if isLower(rs[i-1]) && isUpper(rs[i]) {
				words = append(words, string(rs[start:i]))
				start = i
			}
		}
		words = append(words, string(rs[start:]))
		for _, w := range words {
			if secretWords[strings.ToLower(w)] {
				return true
			}
		}
	}
	return false
}

func isLower(r rune) bool { return r >= 'a' && r <= 'z' }
func isUpper(r rune) bool { return r >= 'A' && r <= 'Z' }

// redactValue keeps surrounding quotes when the original value was quoted,
// so YAML/JSON structure stays valid.
func redactValue(v string) string {
	if len(v) >= 2 {
		if (v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'') {
			return string(v[0]) + Redacted + string(v[0])
		}
	}
	return Redacted
}

// Sanitize replaces anything that looks like a credential or private host
// context with Redacted. A detected kubeconfig blob is neutralized as a
// whole. Sanitize is idempotent: Sanitize(Sanitize(s)) == Sanitize(s).
func Sanitize(s string) string {
	if s == "" {
		return s
	}
	if LooksLikeKubeconfig(s) {
		return Redacted
	}
	out := privKeyRe.ReplaceAllString(s, Redacted)
	// Standalone credential shapes first, so key=value keeps any
	// trailing non-secret context (e.g. "api_key=<redacted> tail=20").
	out = awsKeyRe.ReplaceAllString(out, Redacted)
	out = ghTokenRe.ReplaceAllString(out, Redacted)
	out = ghPatRe.ReplaceAllString(out, Redacted)
	out = slackRe.ReplaceAllString(out, Redacted)
	out = apiKeyRe.ReplaceAllString(out, Redacted)
	out = jwtRe.ReplaceAllString(out, Redacted)
	out = bearerRe.ReplaceAllString(out, Redacted)
	out = kvRe.ReplaceAllStringFunc(out, func(m string) string {
		loc := kvRe.FindStringSubmatchIndex(m)
		// groups: 1=key 2=sep 3=value
		key := m[loc[2]:loc[3]]
		sep := m[loc[4]:loc[5]]
		val := m[loc[6]:loc[7]]
		if !isSecretKey(key) {
			return m
		}
		return key + sep + redactValue(val)
	})
	out = urlAuthRe.ReplaceAllString(out, "${1}"+Redacted+"@")
	out = ipv4PrivRe.ReplaceAllString(out, Redacted)
	out = localHostRe.ReplaceAllString(out, Redacted)
	return out
}
