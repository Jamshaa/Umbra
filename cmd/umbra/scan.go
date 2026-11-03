package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	sigsyaml "sigs.k8s.io/yaml"

	"github.com/Jamshaa/Umbra/api/v1alpha1"
	"github.com/Jamshaa/Umbra/internal/evidence"
	"github.com/Jamshaa/Umbra/internal/livekube"
	"github.com/Jamshaa/Umbra/internal/scanner"
)

func kubeClient(cfg *rest.Config, scheme *runtime.Scheme) (client.Client, error) {
	return client.New(cfg, client.Options{Scheme: scheme})
}

// scanHit is one detector result with its stable rule slug. Scanner
// findings carry no rule id, so the slug comes from the detector table
// below (one slug per detector, in Scan order).
type scanHit struct {
	Rule    string
	Finding v1alpha1.Finding
}

type detectorEntry struct {
	rule string
	fn   func([]byte) []v1alpha1.Finding
}

var detectors = []detectorEntry{
	{"f01", scanner.DetectNoResources},
	{"f02", scanner.DetectTinyMemoryLimit},
	{"f03", scanner.DetectHugeRequests},
	{"f05", scanner.DetectNoLivenessProbe},
	{"f06", scanner.DetectNoReadinessProbe},
	{"f21", scanner.DetectMissingImagePullSecrets},
	{"cron-missing-deadline", scanner.DetectCronMissingDeadline},
	{"cron-short-deadline", scanner.DetectCronShortDeadline},
	{"cron-suspended", scanner.DetectCronSuspended},
	{"cron-overlap", scanner.DetectCronOverlap},
	{"cron-missing-timezone", scanner.DetectCronMissingTimezone},
	{"cron-missing-job-bounds", scanner.DetectCronMissingJobBounds},
	{"cron-history-bloat", scanner.DetectCronHistoryBloat},
	{"cron-gpu-without-placement", scanner.DetectCronGPUWithoutPlacement},
	{"identical-probes", scanner.DetectIdenticalProbes},
	{"single-replica", scanner.DetectSingleReplica},
	{"no-anti-affinity", scanner.DetectNoAntiAffinity},
	{"risky-max-unavailable", scanner.DetectRiskyMaxUnavailable},
	{"insecure-container", scanner.DetectInsecureContainer},
	{"statefulset-partition", scanner.DetectStatefulSetPartition},
	{"job-no-bounds", scanner.DetectJobNoBounds},
	{"service-no-endpoints", scanner.DetectServiceNoEndpoints},
	{"hpa-without-requests", scanner.DetectHPAWithoutRequests},
	{"namespace-no-policy", scanner.DetectNamespaceNoPolicy},
	{"namespace-no-quota", scanner.DetectNamespaceNoQuota},
	{"rbac-wildcard", scanner.DetectRBACWildcard},
	{"hpa-inverted-bounds", scanner.DetectHPAInvertedBounds},
}

// scanBundle runs every detector over one bundle and tags each finding
// with its rule slug. Deterministic order: detector table order.
func scanBundle(docs []byte) []scanHit {
	var out []scanHit
	for _, d := range detectors {
		for _, f := range d.fn(docs) {
			out = append(out, scanHit{Rule: d.rule, Finding: f})
		}
	}
	return out
}

// findingName is the deterministic DNS name for one published finding:
// <rule>-<namespace>-<name> lowercased, hash-suffixed past 253 chars.
func findingName(rule, namespace, name string) string {
	raw := strings.ToLower(rule + "-" + namespace + "-" + name)
	var b strings.Builder
	lastDash := false
	for _, r := range raw {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
		} else if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	s := strings.Trim(b.String(), "-")
	if len(s) > 253 {
		sum := sha256.Sum256([]byte(s))
		s = strings.Trim(s[:242], "-") + "-" + hex.EncodeToString(sum[:])[:10]
	}
	return s
}

const maxFieldBytes = 262144

func truncateField(s string) string {
	if len(s) > maxFieldBytes {
		return s[:maxFieldBytes]
	}
	return s
}

// publishFindings creates or refreshes Finding objects for hits.
// Idempotent: an existing Finding in a later state is never degraded
// or overwritten; a Detected one is refreshed in place.
func publishFindings(ctx context.Context, c client.Client, hits []scanHit, manifests map[string]string) error {
	for _, h := range hits {
		spec := h.Finding.Spec
		name := findingName(h.Rule, spec.TargetRef.Namespace, spec.TargetRef.Name)
		manifest := manifests[spec.TargetRef.Kind+"/"+spec.TargetRef.Namespace+"/"+spec.TargetRef.Name]
		var existing v1alpha1.Finding
		err := c.Get(ctx, client.ObjectKey{Name: name}, &existing)
		if err == nil {
			if existing.Spec.State != "" && existing.Spec.State != v1alpha1.FindingStateDetected {
				continue // never degrade a finding that already moved on
			}
			diagnosis := truncateField(evidence.Sanitize(h.Rule + " detected on " + spec.TargetRef.Kind + " " + spec.TargetRef.Namespace + "/" + spec.TargetRef.Name))
			manifestSan := truncateField(evidence.Sanitize(manifest))
			// Idempotent republish: nothing to change, no write at all.
			if existing.Spec.Severity == spec.Severity && existing.Spec.Category == spec.Category &&
				existing.Spec.TargetRef == spec.TargetRef && existing.Spec.Detector == spec.Detector &&
				existing.Spec.Diagnosis == diagnosis && existing.Spec.PrePatchManifest == manifestSan &&
				existing.Labels["umbra.io/source"] == "scan" {
				continue
			}
			existing.Spec.Severity = spec.Severity
			existing.Spec.Category = spec.Category
			existing.Spec.TargetRef = spec.TargetRef
			existing.Spec.Detector = spec.Detector
			existing.Spec.State = v1alpha1.FindingStateDetected
			existing.Spec.Diagnosis = diagnosis
			existing.Spec.PrePatchManifest = manifestSan
			if existing.Labels == nil {
				existing.Labels = map[string]string{}
			}
			existing.Labels["umbra.io/source"] = "scan"
			if err := c.Update(ctx, &existing); err != nil {
				return fmt.Errorf("update finding %s: %w", name, err)
			}
			continue
		}
		if client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("get finding %s: %w", name, err)
		}
		f := v1alpha1.Finding{
			Spec: v1alpha1.FindingSpec{
				Severity:         spec.Severity,
				Category:         spec.Category,
				TargetRef:        spec.TargetRef,
				Detector:         spec.Detector,
				State:            v1alpha1.FindingStateDetected,
				Diagnosis:        truncateField(evidence.Sanitize(h.Rule + " detected on " + spec.TargetRef.Kind + " " + spec.TargetRef.Namespace + "/" + spec.TargetRef.Name)),
				PrePatchManifest: truncateField(evidence.Sanitize(manifest)),
			},
		}
		f.Name = name
		f.Labels = map[string]string{"umbra.io/source": "scan"}
		if err := c.Create(ctx, &f); err != nil {
			return fmt.Errorf("create finding %s: %w", name, err)
		}
	}
	return nil
}

// requireKindUmbra refuses to run against any other context. There is
// deliberately no flag to skip this guard.
func requireKindUmbra() error {
	loading := clientcmd.NewDefaultClientConfigLoadingRules()
	cfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loading, &clientcmd.ConfigOverrides{}).RawConfig()
	if err != nil {
		return fmt.Errorf("kubeconfig: %w", err)
	}
	if cfg.CurrentContext != "kind-umbra" {
		return fmt.Errorf("current context is %q, want kind-umbra", cfg.CurrentContext)
	}
	return nil
}

var scanKinds = []string{
	"Deployment", "StatefulSet", "DaemonSet", "Job", "CronJob",
	"Service", "HorizontalPodAutoscaler", "Role", "RoleBinding",
}

func scanMain(args []string) {
	fs := flag.NewFlagSet("scan", flag.ContinueOnError)
	ns := fs.String("namespace", "", "namespace to scan (required)")
	publish := fs.Bool("publish", false, "create or refresh Finding objects")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}
	if *ns == "" {
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
	var docs [][]byte
	manifests := map[string]string{}
	add := func(obj map[string]any) {
		clean := livekube.CleanForPatch(obj)
		raw, err := sigsyaml.Marshal(clean)
		if err != nil {
			return
		}
		docs = append(docs, raw)
		kind, _ := clean["kind"].(string)
		meta, _ := clean["metadata"].(map[string]any)
		ons, _ := meta["namespace"].(string)
		oname, _ := meta["name"].(string)
		manifests[kind+"/"+ons+"/"+oname] = string(raw)
	}
	for _, kind := range scanKinds {
		objs, err := lk.List(ctx, kind, *ns, nil)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		for _, o := range objs {
			add(o)
		}
	}
	if obj, err := lk.Get(ctx, "Namespace", *ns, ""); err == nil {
		add(obj)
	}
	var bundle []byte
	for i, d := range docs {
		if i > 0 {
			bundle = append(bundle, []byte("\n---\n")...)
		}
		bundle = append(bundle, d...)
	}
	hits := scanBundle(bundle)
	fmt.Printf("rule\tseverity\tresource\n")
	for _, h := range hits {
		s := h.Finding.Spec
		fmt.Printf("%s\t%s\t%s %s/%s\n", h.Rule, strings.ToLower(string(s.Severity)), s.TargetRef.Kind, s.TargetRef.Namespace, s.TargetRef.Name)
	}
	if !*publish {
		return
	}
	scheme := runtime.NewScheme()
	if err := v1alpha1.AddToScheme(scheme); err != nil {
		fmt.Fprintln(os.Stderr, "add Umbra scheme:", err)
		os.Exit(1)
	}
	cfg, err := ctrl.GetConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, "kubeconfig:", err)
		os.Exit(1)
	}
	c, err := kubeClient(cfg, scheme)
	if err != nil {
		fmt.Fprintln(os.Stderr, "k8s client:", err)
		os.Exit(1)
	}
	if err := publishFindings(ctx, c, hits, manifests); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	fmt.Printf("published %d findings\n", len(hits))
}
