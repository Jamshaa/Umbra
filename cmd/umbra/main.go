// Command umbra is the human CLI. Subcommands:
//
//	umbra approve FINDING --as USER   record a human approval gate decision
//	umbra receipts verify <chain-file> verify a receipt registry chain
//
// Approval is an explicit human act: the command only writes the
// umbra.dev/approved annotation. The reconciler interprets it (see
// internal/controller) and records who approved and when. kubectl can
// do the same by hand:
//
//	kubectl annotate finding <id> umbra.dev/approved=<user>
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/Jamshaa/Umbra/api/v1alpha1"
	"github.com/Jamshaa/Umbra/internal/controller"
)

func usage() {
	fmt.Fprintln(os.Stderr, "usage: umbra <command>")
	fmt.Fprintln(os.Stderr, "  approve FINDING --as USER    record a human approval")
	fmt.Fprintln(os.Stderr, "  receipts verify <chain-file> verify a receipt registry chain")
}

// approve records the human approval on the Finding. It validates the
// identity with the same rule the reconciler enforces, so the CLI can
// never write an annotation the controller would ignore.
func approve(ctx context.Context, c client.Client, name, user string) error {
	if _, ok := controller.ParseApproval(map[string]string{controller.AnnotationApproved: user}); !ok {
		return fmt.Errorf("invalid user %q: must be non-empty without whitespace", user)
	}
	var f v1alpha1.Finding
	if err := c.Get(ctx, types.NamespacedName{Name: name}, &f); err != nil {
		return fmt.Errorf("get finding %q: %w", name, err)
	}
	if f.Annotations == nil {
		f.Annotations = map[string]string{}
	}
	f.Annotations[controller.AnnotationApproved] = user
	if err := c.Update(ctx, &f); err != nil {
		return fmt.Errorf("annotate finding %q: %w", name, err)
	}
	return nil
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "receipts":
		if len(os.Args) != 4 || os.Args[2] != "verify" {
			receiptsUsage()
			os.Exit(2)
		}
		if err := verifyChainFile(os.Args[3]); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		return
	case "approve":
		approveMain(os.Args[2:])
	default:
		usage()
		os.Exit(2)
	}
}

func approveMain(args []string) {
	fs := flag.NewFlagSet("approve", flag.ContinueOnError)
	as := fs.String("as", "", "approver identity (required)")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(os.Args[2:]); err != nil {
		os.Exit(2)
	}
	if fs.NArg() != 1 || *as == "" {
		usage()
		os.Exit(2)
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
	c, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		fmt.Fprintln(os.Stderr, "k8s client:", err)
		os.Exit(1)
	}
	name := fs.Arg(0)
	if err := approve(context.Background(), c, name, *as); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	fmt.Printf("finding %q approved by %s\n", name, *as)
}
