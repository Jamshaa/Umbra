package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Jamshaa/Umbra/internal/attribution"
	"github.com/Jamshaa/Umbra/internal/evidence"
	"github.com/Jamshaa/Umbra/internal/livekube"
)

// changesReport renders who changed what for one object: managedFields
// attribution first, then (with --audit-file) matching audit actors.
// Every printed identity passes through the sanitizer.
func changesReport(obj map[string]any, auditLines []string, kind, name, namespace string) []string {
	var out []string
	for _, a := range attribution.Attribute(obj) {
		when := ""
		if !a.UpdatedAt.IsZero() {
			when = a.UpdatedAt.UTC().Format(time.RFC3339)
		}
		out = append(out, evidence.Sanitize(fmt.Sprintf("%s\t%s\t%s", a.Field, a.Manager, when)))
	}
	// Map kind to the audit objectRef resource plural (same table the
	// live adapter uses; unknown kinds simply match nothing).
	resource := map[string]string{
		"Deployment": "deployments", "StatefulSet": "statefulsets",
		"DaemonSet": "daemonsets", "ReplicaSet": "replicasets",
		"Pod": "pods", "Service": "services", "CronJob": "cronjobs",
		"Job": "jobs",
	}[kind]
	for _, line := range auditLines {
		ev, err := attribution.ParseAuditLine([]byte(line))
		if err != nil {
			continue
		}
		if resource == "" || ev.Resource != resource || ev.Name != name || ev.Namespace != namespace {
			continue
		}
		identity, qualifier := ev.Present()
		level := attribution.Resolve(&ev, nil).Level.String()
		when := ""
		if !ev.Timestamp.IsZero() {
			when = ev.Timestamp.UTC().Format(time.RFC3339)
		}
		out = append(out, evidence.Sanitize(fmt.Sprintf("%s\t%s (%s)\t%s\t%s", identity, qualifier, level, ev.Verb, when)))
	}
	return out
}

func changesMain(args []string) {
	fs := flag.NewFlagSet("changes", flag.ContinueOnError)
	kind := fs.String("kind", "", "object kind (required)")
	name := fs.String("name", "", "object name (required)")
	ns := fs.String("namespace", "", "object namespace (required)")
	auditFile := fs.String("audit-file", "", "local audit log file (JSON lines)")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}
	if *kind == "" || *name == "" || *ns == "" {
		usage()
		os.Exit(2)
	}
	if err := requireKindUmbra(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	ctx := context.Background()
	lk, err := livekube.New("")
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	obj, err := lk.Get(ctx, *kind, *name, *ns)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	var auditLines []string
	if *auditFile != "" {
		data, err := os.ReadFile(*auditFile)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		auditLines = strings.Split(string(data), "\n")
	}
	lines := changesReport(obj, auditLines, *kind, *name, *ns)
	if len(lines) == 0 {
		fmt.Println("no changes recorded")
		return
	}
	for _, l := range lines {
		fmt.Println(l)
	}
}
