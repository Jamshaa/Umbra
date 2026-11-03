package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Jamshaa/Umbra/internal/brain"
	"github.com/Jamshaa/Umbra/internal/receipt"
)

// appendReceipt adds one signed receipt to the chain file. It fails
// closed without a key: an unsigned chain in this mode proves nothing.
// The file is created (with parent dirs) when missing.
func appendReceipt(path string, in receipt.Input, key []byte) error {
	if len(key) == 0 {
		return fmt.Errorf("receipt: UMBRA_RECEIPT_HMAC_KEY is required: set it to a fresh random value in this shell only (see SECURITY.md); never store it in a file")
	}
	var chain []string
	if data, err := os.ReadFile(path); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if strings.TrimSpace(line) != "" {
				chain = append(chain, line)
			}
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("read receipt chain: %w", err)
	}
	prev := "GENESIS"
	if len(chain) > 0 {
		prev = receipt.Fingerprint(chain[len(chain)-1])
	}
	in.PrevHash = prev
	rec := receipt.Build(in)
	chain = receipt.AppendRegistroSigned(chain, rec.JSON, key)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir receipt chain: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open receipt chain: %w", err)
	}
	defer f.Close()
	_, err = fmt.Fprintln(f, chain[len(chain)-1])
	return err
}

// runPing performs the minimal model call: one fake get_manifest tool
// offer, and prints only output item types, the function_call name when
// present, and token counts. Never the key, never headers.
func runPing(ctx context.Context, c *brain.Client) (string, error) {
	tools := []brain.Tool{{
		Name:        "get_manifest",
		Description: "ping probe: read one manifest",
		Parameters:  []byte(`{"type":"object","properties":{"kind":{"type":"string"},"name":{"type":"string"},"namespace":{"type":"string"}},"required":["kind","name","namespace"]}`),
	}}
	res, err := c.Call(ctx, []brain.InputItem{brain.UserMessage("ping: call get_manifest once for Deployment web in demo")}, tools, "")
	if err != nil {
		return "", err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "output_types: %s\n", strings.Join(res.OutputTypes, ","))
	if len(res.Calls) > 0 {
		fmt.Fprintf(&b, "function_call: %s\n", res.Calls[0].Name)
	} else {
		b.WriteString("function_call: none\n")
	}
	fmt.Fprintf(&b, "input_tokens: %d\noutput_tokens: %d\n", res.Usage.InputTokens, res.Usage.OutputTokens)
	return b.String(), nil
}

func pingMain() {
	cfg, err := brain.FromEnv()
	if err != nil {
		fmt.Fprintln(os.Stderr, "umbra-agent ping:", err)
		os.Exit(1)
	}
	out, err := runPing(context.Background(), brain.New(cfg))
	if err != nil {
		fmt.Fprintln(os.Stderr, "umbra-agent ping:", err)
		os.Exit(1)
	}
	fmt.Print(out)
}
