package evidence

import (
	"strconv"
	"strings"
	"testing"
)

func TestSanitizeTokensAndKeys(t *testing.T) {
	cases := []struct {
		name  string
		in    string
		check func(t *testing.T, out string)
	}{
		{
			name: "api key assignment",
			in:   "get_logs failed: api_key=sk-abcdefghij1234567890XYZ extra",
			check: func(t *testing.T, out string) {
				t.Helper()
				if strings.Contains(out, "sk-abcdefghij1234567890XYZ") {
					t.Errorf("secret leaked: %q", out)
				}
				if !strings.Contains(out, Redacted) {
					t.Errorf("missing placeholder: %q", out)
				}
			},
		},
		{
			name: "password yaml",
			in:   "password: hunter2-hunter2",
			check: func(t *testing.T, out string) {
				t.Helper()
				if strings.Contains(out, "hunter2") {
					t.Errorf("secret leaked: %q", out)
				}
			},
		},
		{
			name: "quoted token json",
			in:   `"token": "ghp_abcdefghij1234567890abcdefghij12345678"`,
			check: func(t *testing.T, out string) {
				t.Helper()
				if strings.Contains(out, "ghp_") {
					t.Errorf("secret leaked: %q", out)
				}
				if !strings.Contains(out, `"`+Redacted+`"`) {
					t.Errorf("quotes not preserved: %q", out)
				}
			},
		},
		{name: "aws access key", in: "AKIAIOSFODNN7EXAMPLE leaked here",
			check: func(t *testing.T, out string) {
				t.Helper()
				if strings.Contains(out, "AKIAIOSFODNN7EXAMPLE") {
					t.Errorf("secret leaked: %q", out)
				}
			}},
		{name: "bearer header", in: "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9H0s7Y6i3745s",
			check: func(t *testing.T, out string) {
				t.Helper()
				if strings.Contains(out, "eyJhbGci") {
					t.Errorf("secret leaked: %q", out)
				}
			}},
		{name: "jwt blob", in: "saw eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9H0s7Y6i3745s in logs",
			check: func(t *testing.T, out string) {
				t.Helper()
				if strings.Contains(out, "eyJhbGci") {
					t.Errorf("secret leaked: %q", out)
				}
			}},
		{name: "private key block", in: "key:\n-----BEGIN RSA PRIVATE KEY-----\nMIIBOgIBAAJBAKc2\n-----END RSA PRIVATE KEY-----",
			check: func(t *testing.T, out string) {
				t.Helper()
				if strings.Contains(out, "BEGIN RSA PRIVATE KEY") {
					t.Errorf("secret leaked: %q", out)
				}
			}},
		{name: "url with credentials", in: "fetch https://admin:s3cret@example.com/manifest failed",
			check: func(t *testing.T, out string) {
				t.Helper()
				if strings.Contains(out, "s3cret") {
					t.Errorf("secret leaked: %q", out)
				}
				if !strings.Contains(out, "https://"+Redacted+"@example.com/manifest") {
					t.Errorf("url not neutralized: %q", out)
				}
			}},
		{name: "private ip", in: "dial tcp 10.0.4.15:8081: connection refused",
			check: func(t *testing.T, out string) {
				t.Helper()
				if strings.Contains(out, "10.0.4.15") {
					t.Errorf("private host leaked: %q", out)
				}
			}},
		{name: "private hostname", in: "probe failed on db-master.internal:5432",
			check: func(t *testing.T, out string) {
				t.Helper()
				if strings.Contains(out, "db-master.internal") {
					t.Errorf("private host leaked: %q", out)
				}
			}},
		{name: "secret untouched key names", in: "monkey: banana\nmatchLabels:\n  app: web",
			check: func(t *testing.T, out string) {
				t.Helper()
				if out != "monkey: banana\nmatchLabels:\n  app: web" {
					t.Errorf("non-secret keys altered: %q", out)
				}
			}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.check(t, Sanitize(tc.in))
		})
	}
}

func TestSanitizeKubeconfig(t *testing.T) {
	blob := `apiVersion: v1
kind: Config
clusters:
- name: kind-umbra
  cluster:
    certificate-authority-data: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0t
    server: https://127.0.0.1:6443
users:
- name: kind-umbra
  user:
    client-certificate-data: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0t
    client-key-data: LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVkt
current-context: kind-umbra`
	if got := Sanitize(blob); got != Redacted {
		t.Fatalf("kubeconfig not neutralized as a whole: %q", got)
	}
	if !LooksLikeKubeconfig(blob) {
		t.Fatal("LooksLikeKubeconfig missed a kubeconfig")
	}
	if LooksLikeKubeconfig("kind: Deployment\nclusters: 2\n") {
		t.Fatal("LooksLikeKubeconfig false positive on Deployment")
	}
}

func TestSanitizeStructuralYAMLUntouched(t *testing.T) {
	// Publishable-by-design manifest: must survive byte-identical.
	in := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: web-frontend
  namespace: demo
  labels:
    app.kubernetes.io/name: web-frontend
spec:
  replicas: 2
  selector:
    matchLabels:
      app.kubernetes.io/name: web-frontend
  template:
    spec:
      containers:
        - name: web
          image: ghcr.io/example/web:1.4.2
          ports:
            - containerPort: 8080
              name: http
          livenessProbe:
            httpGet:
              path: /healthz
              port: http`
	if got := Sanitize(in); got != in {
		t.Fatalf("structural YAML altered:\n%s", diffLines(in, got))
	}
}

func TestSanitizeIdempotent(t *testing.T) {
	in := "token: hunter2-hunter2 and 10.1.2.3 plus AKIAIOSFODNN7EXAMPLE"
	once := Sanitize(in)
	twice := Sanitize(once)
	if once != twice {
		t.Fatalf("not idempotent:\n%q\n%q", once, twice)
	}
	if Sanitize(Redacted) != Redacted {
		t.Fatal("placeholder itself is not stable")
	}
}

func TestFingerprint(t *testing.T) {
	a := Fingerprint("prompt\nresult")
	b := Fingerprint("prompt\nresult")
	c := Fingerprint("prompt\nchanged")
	if a != b || len(a) != 64 {
		t.Fatalf("not deterministic hex sha256: %q", a)
	}
	if a == c {
		t.Fatal("different inputs share a fingerprint")
	}
}

func diffLines(a, b string) string {
	la, lb := strings.Split(a, "\n"), strings.Split(b, "\n")
	var sb strings.Builder
	for i := range max(len(la), len(lb)) {
		var x, y string
		if i < len(la) {
			x = la[i]
		}
		if i < len(lb) {
			y = lb[i]
		}
		if x != y {
			sb.WriteString("line " + strconv.Itoa(i+1) + "\n")
			sb.WriteString("- " + x + "\n+ " + y + "\n")
		}
	}
	return sb.String()
}
