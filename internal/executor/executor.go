package executor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// PRResult is what a run produced.
type PRResult struct {
	Branch   string
	Commit   string
	PRUrl    string // empty when no token (plan only)
	PlanOnly bool
	Diff     string
}

// Runner runs git/gh. Injected so tests do not touch the real repo or network.
type Runner func(name string, dir string, args ...string) (string, error)

// OpenPR creates a branch, commits the patched files and (if GH_TOKEN exists)
// opens a pull request. Without a token it returns the plan (branch + diff)
// without any network access.
func OpenPR(repoDir string, findingName string, files map[string]string, receipt []byte, runner Runner) (*PRResult, error) {
	if runner == nil {
		runner = defaultRunner
	}
	branch := "umbra/" + findingName
	res := &PRResult{Branch: branch}

	// 1. Create/reset a clean branch from the current HEAD.
	if _, err := runner("git", repoDir, "checkout", "-b", branch); err != nil {
		if _, err2 := runner("git", repoDir, "checkout", branch); err2 != nil {
			return nil, fmt.Errorf("executor: checkout %s: %w", branch, err)
		}
		if _, err := runner("git", repoDir, "reset", "--hard", "HEAD"); err != nil {
			return nil, fmt.Errorf("executor: reset: %w", err)
		}
	}

	// 2. Write the patched files.
	for rel, content := range files {
		full := filepath.Join(repoDir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return nil, fmt.Errorf("executor: mkdir: %w", err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			return nil, fmt.Errorf("executor: write %s: %w", rel, err)
		}
	}

	// 3. Stage + commit.
	if _, err := runner("git", repoDir, "add", "-A"); err != nil {
		return nil, fmt.Errorf("executor: git add: %w", err)
	}
	msg := "fix(umbra): restore compliance for " + findingName
	if _, err := runner("git", repoDir, "commit", "-m", msg, "--author", "Umbra <umbra@localhost>"); err != nil {
		return nil, fmt.Errorf("executor: commit: %w", err)
	}
	out, _ := runner("git", repoDir, "rev-parse", "HEAD")
	res.Commit = strings.TrimSpace(out)

	// 4. Diff against the parent for the receipt.
	diff, _ := runner("git", repoDir, "diff", "HEAD~1", "HEAD")
	res.Diff = diff

	token := os.Getenv("GH_TOKEN")
	if token == "" {
		res.PlanOnly = true
		return res, nil
	}

	// 5. Open the PR (only when a token exists). The token travels only in
	// the process env of the child, never in argv or logs.
	body := string(receipt)
	title := "fix(umbra): restore compliance for " + findingName
	prOut, err := runner("gh", repoDir, "pr", "create", "--title", title, "--body", body, "--head", branch)
	if err != nil {
		return nil, fmt.Errorf("executor: gh pr create: %w", err)
	}
	res.PRUrl = strings.TrimSpace(prOut)
	return res, nil
}

func defaultRunner(name, dir string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// keep sort import used even when files map is unordered.
var _ = sort.Strings
