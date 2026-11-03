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
	fmt.Fprintln(os.Stderr, "UMBRA_RECEIPT_HMAC_KEY is required; every line must carry")
	fmt.Fprintln(os.Stderr, "a valid HMAC signature (see AppendRegistroSigned).")
}

// verifyChainFile checks a registry file line by line. Blank lines are
// skipped so trailing newlines don't fail verification. It prints the
// result in plain language and returns nil only when every link holds.
// Receipt verification fails closed when UMBRA_RECEIPT_HMAC_KEY is absent:
// hash links alone are not authenticating because an attacker can recompute
// them after rewriting the file.
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
	key := os.Getenv("UMBRA_RECEIPT_HMAC_KEY")
	if key == "" {
		return fmt.Errorf("receipt: UMBRA_RECEIPT_HMAC_KEY is required for authenticated verification")
	}
	if err := receipt.VerifyChainSigned(chain, []byte(key)); err != nil {
		return err
	}
	fmt.Printf("chain intact: %d signed lines verified\n", len(chain))
	return nil
}
