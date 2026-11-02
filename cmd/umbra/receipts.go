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
	fmt.Fprintln(os.Stderr, "With UMBRA_RECEIPT_HMAC_KEY set, every line must carry a")
	fmt.Fprintln(os.Stderr, "valid HMAC signature (see AppendRegistroSigned).")
}

// verifyChainFile checks a registry file line by line. Blank lines are
// skipped so trailing newlines don't fail verification. It prints the
// result in plain language and returns nil only when every link holds.
// With UMBRA_RECEIPT_HMAC_KEY set, signatures are verified too: a
// rewritten chain without the key is rejected even if links are intact.
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
	if key := os.Getenv("UMBRA_RECEIPT_HMAC_KEY"); key != "" {
		if err := receipt.VerifyChainSigned(chain, []byte(key)); err != nil {
			return err
		}
		fmt.Printf("chain intact: %d signed lines verified\n", len(chain))
		return nil
	}
	if err := receipt.VerifyChain(chain); err != nil {
		return err
	}
	fmt.Printf("chain intact: %d lines verified\n", len(chain))
	return nil
}
