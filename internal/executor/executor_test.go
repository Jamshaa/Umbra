package executor

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenPRPlanOnly(t *testing.T) {
	dir := t.TempDir()
	// init a git repo with one commit
	g := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	g("init", "-q")
	g("config", "user.email", "a@b.c")
	g("config", "user.name", "x")
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	g("add", ".")
	g("commit", "-qm", "init")

	// runner that records calls and never touches network
	var calls []string
	runner := func(name, d string, args ...string) (string, error) {
		calls = append(calls, name+" "+stringsJoin(args))
		if name == "git" && len(args) > 0 && args[0] == "rev-parse" {
			return "deadbeef\n", nil
		}
		if name == "git" && len(args) > 0 && args[0] == "diff" {
			return "diff --git a/f.txt b/f.txt\n", nil
		}
		return "", nil
	}
	_ = context.Background
	res, err := OpenPR(dir, "f01", map[string]string{"f.txt": "patched"}, []byte("receipt"), runner)
	if err != nil {
		t.Fatalf("OpenPR: %v", err)
	}
	if !res.PlanOnly {
		t.Fatal("expected PlanOnly without GH_TOKEN")
	}
	if res.Branch != "umbra/f01" || res.Commit != "deadbeef" {
		t.Fatalf("res = %+v", res)
	}
	if res.PRUrl != "" {
		t.Fatalf("PRUrl should be empty in plan mode, got %q", res.PRUrl)
	}
	if res.Diff == "" {
		t.Fatal("diff missing")
	}
	// no gh invocation in plan mode
	for _, c := range calls {
		if strings.HasPrefix(c, "gh ") {
			t.Fatalf("gh called in plan mode: %s", c)
		}
	}
}

func TestOpenPRWithToken(t *testing.T) {
	t.Setenv("GH_TOKEN", "secret-token")
	dir := t.TempDir()
	g := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	g("init", "-q")
	g("config", "user.email", "a@b.c")
	g("config", "user.name", "x")
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	g("add", ".")
	g("commit", "-qm", "init")

	var ghCalled bool
	runner := func(name, d string, args ...string) (string, error) {
		if name == "git" && len(args) > 0 && args[0] == "rev-parse" {
			return "cafebabe\n", nil
		}
		if name == "git" && len(args) > 0 && args[0] == "diff" {
			return "diff text\n", nil
		}
		if name == "gh" {
			ghCalled = true
			if len(args) < 4 || args[0] != "pr" || args[1] != "create" {
				t.Fatalf("bad gh args: %v", args)
			}
			return "https://github.com/x/y/pull/1\n", nil
		}
		return "", nil
	}
	res, err := OpenPR(dir, "f02", map[string]string{"f.txt": "patched"}, []byte("receipt"), runner)
	if err != nil {
		t.Fatalf("OpenPR: %v", err)
	}
	if res.PlanOnly {
		t.Fatal("expected PR mode with GH_TOKEN")
	}
	if res.PRUrl != "https://github.com/x/y/pull/1" {
		t.Fatalf("PRUrl = %q", res.PRUrl)
	}
	if !ghCalled {
		t.Fatal("gh was not called")
	}
}

func stringsJoin(args []string) string {
	out := ""
	for i, a := range args {
		if i > 0 {
			out += " "
		}
		out += a
	}
	return out
}
