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

func TestSanitizeExtendedPatterns(t *testing.T) {
	cases := []struct {
		name string
		in   string
		leak string // raw value that must not survive
		keep string // fragment that must survive ("" to skip)
	}{
		{"long base64 blob", "key is dGhpcy1pcy1hLXNlY3JldC10b2tlbi12YWx1ZS0xMjM0NQ==", "dGhpcy1pcy1hLXNlY3JldC10b2tlbi12YWx1ZS0xMjM0NQ==", ""},
		{"base64 with padding", "secret: QUJDREVGR0hJSktMTU5PUFFSU1RVVldYWQ==", "QUJDREVGR0hJSktMTU5PUFFSU1RVVldYWQ==", ""},
		{"hex digest untouched", "sha256: e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", "", "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"},
		{"uuid untouched", "id: 550e8400-e29b-41d4-a716-446655440000", "", "550e8400"},
		{"plain word untouched", "status: initialized successfully", "", "initialized"},
		{"query token", "GET https://hooks.example.com/hook?token=abc123XYZ789&ok=1 failed", "abc123XYZ789", "hooks.example.com/hook"},
		{"query api key", "callback https://api.example.com/x?api_key=KEY-9f8e7d6c5b4a HTTP/1.1", "KEY-9f8e7d6c5b4a", ""},
		{"cluster.local host", "dial web.demo.svc.cluster.local:8080 refused", "web.demo.svc.cluster.local", ""},
		{"corp host", "ldap corp-dc01.corp:389 timeout", "corp-dc01.corp", ""},
		{"compound env name", "dbpassword: s3cret-pg-pass", "s3cret-pg-pass", ""},
		{"access token kv", "accessToken: ya29.a0AfH6SMBx9Q2wE", "ya29.a0AfH6SMBx9Q2wE", ""},
		{"client secret yaml", "clientSecret: topsecretvalue1", "topsecretvalue1", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := Sanitize(tc.in)
			if tc.leak != "" && strings.Contains(out, tc.leak) {
				t.Errorf("secret leaked: %q -> %q", tc.in, out)
			}
			if tc.keep != "" && !strings.Contains(out, tc.keep) {
				t.Errorf("harmless text altered: %q -> %q", tc.in, out)
			}
			if out != Sanitize(out) {
				t.Errorf("not idempotent: %q -> %q", out, Sanitize(out))
			}
		})
	}
}

func TestSanitizeEmbeddedJSON(t *testing.T) {
	in := `event: data: {"password":"actual-secret","nested":{"token":"nested-secret"}}`
	out := Sanitize(in)
	for _, secret := range []string{"actual-secret", "nested-secret"} {
		if strings.Contains(out, secret) {
			t.Fatalf("embedded JSON secret leaked: %q -> %q", secret, out)
		}
	}
	if !strings.Contains(out, `"password":"`+Redacted+`"`) ||
		!strings.Contains(out, `"token":"`+Redacted+`"`) {
		t.Fatalf("embedded JSON was not structurally sanitized: %q", out)
	}
	if out != Sanitize(out) {
		t.Fatalf("embedded JSON sanitization is not idempotent: %q -> %q", out, Sanitize(out))
	}
}

// TestNoSecretReachesModelBody is the T4 gate: a battery of realistic
// secrets goes through Sanitize exactly as tool output would before a
// model call, and none of the raw values may survive in the result.
func TestNoSecretReachesModelBody(t *testing.T) {
	planted := []string{
		"AKIAIOSFODNN7EXAMPLE",
		"wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
		"ghp_abcdefghij1234567890abcdefghij12345678",
		"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9H0s7Y6i3745s",
		"-----BEGIN RSA PRIVATE KEY-----\nMIIBOgIBAAJBAKc2\n-----END RSA PRIVATE KEY-----",
		"Bearer hunter2-session-token-abc",
		"dGhpcy1pcy1hLXNlY3JldC10b2tlbi12YWx1ZS0xMjM0NQ==", "hunter2-hunter2",
		"mongodb://dbadmin:s3cret-pg-pass@db-master.internal:27017/app?authSource=admin",
		"callback https://hooks.example.com/x?token=tok_live_abc123XYZ",
		"web.demo.svc.cluster.local",
		"10.244.0.15",
		"kind: Config\nclusters: [c]\nusers: [u]\n",
	}
	var body strings.Builder
	body.WriteString("tool get_logs tail=200\nresult:\n")
	for _, p := range planted {
		body.WriteString(p + "\n")
	}
	body.WriteString("end of output\n")
	req := map[string]any{"model": "muse-spark", "input": body.String()}
	sanitized := Sanitize(req["input"].(string))
	for _, p := range planted {
		if strings.Contains(sanitized, p) {
			t.Fatalf("secret reached the model body: %q", p)
		}
	}
	// The noisiest shapes must be gone even as fragments.
	for _, frag := range []string{"AKIAIOSFODNN7EXAMPLE", "BEGIN RSA PRIVATE KEY", "hunter2", "tok_live_abc123XYZ", "cluster.local"} {
		if strings.Contains(sanitized, frag) {
			t.Fatalf("secret fragment reached the model body: %q", frag)
		}
	}
	if !strings.Contains(sanitized, Redacted) {
		t.Fatal("nothing was redacted, gate is vacuous")
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

func TestSanitizeSecretDataKeys(t *testing.T) {
	// data:/stringData: values are always redacted (a Secret's data IS
	// the secret). Deliberate trade-off: innocent ConfigMap data gets
	// redacted too. Confidentiality wins over availability here.
	for _, in := range []string{
		"data:\n  password: hunter2-hunter2",
		"stringData:\n  token: hunter2-hunter2",
		"data: c2VjcmV0LXZhbHVl",
		"data: us-east-1",
	} {
		out := Sanitize(in)
		if strings.Contains(out, "hunter2") || strings.Contains(out, "c2VjcmV0LXZhbHVl") || strings.Contains(out, "us-east-1") {
			t.Fatalf("secret data leaked: %q -> %q", in, out)
		}
		if !strings.Contains(out, Redacted) {
			t.Fatalf("no placeholder emitted: %q", out)
		}
	}
}

func TestSanitizeBlockScalars(t *testing.T) {
	for _, in := range []string{
		"password: |\n  actual-secret\nnext: safe\n",
		"data: |-\n  c2VjcmV0LXZhbHVl\nkind: Secret\n",
		"stringData: >\n  another-secret\n",
	} {
		out := Sanitize(in)
		if strings.Contains(out, "actual-secret") ||
			strings.Contains(out, "c2VjcmV0LXZhbHVl") ||
			strings.Contains(out, "another-secret") {
			t.Fatalf("block scalar secret leaked: %q -> %q", in, out)
		}
		if !strings.Contains(out, Redacted) {
			t.Fatalf("no placeholder emitted: %q -> %q", in, out)
		}
		if out != Sanitize(out) {
			t.Fatalf("not idempotent: %q -> %q", out, Sanitize(out))
		}
	}
}

func TestSanitizeNestedJSONSecrets(t *testing.T) {
	cases := []struct {
		name string
		in   string
		leak string
	}{
		{
			name: "nested object",
			in:   `{"metadata":{"annotations":{"note":"ok"}}, "config":{"password":"hunter2"}}`,
			leak: "hunter2",
		},
		{
			name: "json embedded in config value",
			in:   `{"data":{"config.json":"{\"database\":{\"clientSecret\":\"topsecret\"}}"}}`,
			leak: "topsecret",
		},
		{
			name: "nested array",
			in:   `{"items":[{"credentials":{"token":"tok_live_abc123"}}]}`,
			leak: "tok_live_abc123",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := Sanitize(tc.in)
			if strings.Contains(out, tc.leak) {
				t.Fatalf("nested secret leaked: %q -> %q", tc.in, out)
			}
			if !strings.Contains(out, Redacted) {
				t.Fatalf("no placeholder emitted: %q -> %q", tc.in, out)
			}
			if out != Sanitize(out) {
				t.Fatalf("not idempotent: %q -> %q", out, Sanitize(out))
			}
		})
	}
}

func TestSanitizeEnvSecrets(t *testing.T) {
	// K8s env entries keep the credential in a generic "value" field with
	// the secret identity in the sibling "name" field. Only the value is
	// redacted; the name stays as diagnosis context.
	for _, tc := range []struct {
		name string
		in   string
		want string
	}{
		{
			"block short plaintext",
			"        - name: DB_PASSWORD\n          value: hunter2-hunter2\n",
			"        - name: DB_PASSWORD\n          value: <REPLACE-ME>\n",
		},
		{
			"block quoted",
			"        - name: api_key\n          value: \"s3cret-pg-pass\"\n",
			"        - name: api_key\n          value: \"<REPLACE-ME>\"\n",
		},
		{
			"block innocent untouched",
			"        - name: PORT\n          value: 8080\n",
			"        - name: PORT\n          value: 8080\n",
		},
		{
			"block valueFrom reference untouched",
			"        - name: DB_PASSWORD\n          valueFrom:\n            secretKeyRef:\n              name: db-creds\n",
			"        - name: DB_PASSWORD\n          valueFrom:\n            secretKeyRef:\n              name: db-creds\n",
		},
		{
			"json order",
			`{"name":"DB_PASSWORD","value":"hunter2-hunter2"}`,
			`{"name":"DB_PASSWORD","value":"<REPLACE-ME>"}`,
		},
		{
			"json reversed order",
			`{"value":"hunter2-hunter2","name":"TOKEN"}`,
			`{"value":"<REPLACE-ME>","name":"TOKEN"}`,
		},
		{
			"json innocent untouched",
			`{"name":"PORT","value":"8080"}`,
			`{"name":"PORT","value":"8080"}`,
		},
		{
			"flow secret",
			`env: [{name: DB_PASSWORD, value: hunter2-hunter2}]`,
			`env: [{name: DB_PASSWORD, value: <REPLACE-ME>}]`,
		},
		{
			"flow quoted secret",
			`{name: "api_key", value: "s3cret-pg-pass"}`,
			`{name: "api_key", value: "<REPLACE-ME>"}`,
		},
		{
			"flow reversed secret",
			`{value: hunter2-hunter2, name: TOKEN}`,
			`{value: <REPLACE-ME>, name: TOKEN}`,
		},
		{
			"flow innocent untouched",
			`{name: PORT, value: 8080}`,
			`{name: PORT, value: 8080}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := Sanitize(tc.in); got != tc.want {
				t.Fatalf("got  %q\nwant %q", got, tc.want)
			}
		})
	}
	// Redacted output must be a fixed point.
	once := Sanitize("        - name: DB_PASSWORD\n          value: hunter2-hunter2\n")
	if twice := Sanitize(once); twice != once {
		t.Fatalf("not idempotent:\n%q\n%q", once, twice)
	}
	onceJSON := Sanitize(`{"name":"DB_PASSWORD","value":"hunter2-hunter2"}`)
	if twice := Sanitize(onceJSON); twice != onceJSON {
		t.Fatalf("json not idempotent:\n%q\n%q", onceJSON, twice)
	}
	onceFlow := Sanitize(`{name: DB_PASSWORD, value: hunter2-hunter2}`)
	if twice := Sanitize(onceFlow); twice != onceFlow {
		t.Fatalf("flow not idempotent:\n%q\n%q", onceFlow, twice)
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
