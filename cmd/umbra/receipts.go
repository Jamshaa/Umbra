package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/Jamshaa/Umbra/internal/receipt"
)

func receiptsUsage() {
	fmt.Fprintln(os.Stderr, "usage: umbra receipts verify <chain-file>")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Verifies a receipt registry: one JSON chain line per line,")
	fmt.Fprintln(os.Stderr, "as written by the registry (see internal/receipt).")
}

// verifyChainFile checks a registry file line by line. Blank lines are
// skipped so trailing newlines don't fail verification. It prints the
// result in plain language and returns nil only when every link holds.
func verifyChainFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	var chain []string
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		chain = append(chain, line)
	}
	if err := receipt.VerifyChain(chain); err != nil {
		return err
	}
	fmt.Printf("chain intact: %d lines verified\n", len(chain))
	return nil
}
