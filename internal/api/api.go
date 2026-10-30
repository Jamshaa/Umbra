package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/Jamshaa/Umbra/api/v1alpha1"
	"github.com/Jamshaa/Umbra/internal/evidence"
)

// FindingView is the UI shape documented in ui/src/api.js.
type FindingView struct {
	ID          string   `json:"id"`
	Severity    string   `json:"severity"`
	State       string   `json:"state"`
	Kind        string   `json:"kind"`
	Target      string   `json:"target"`
	LiveYaml    string   `json:"liveYaml"`
	DiffRemoved []string `json:"diffRemoved"`
	DiffAdded   []string `json:"diffAdded"`
}

// NewHandler wires the read-only routes. distDir is served at /;
// a missing bundle directory answers 503 with a plain message.
func NewHandler(c client.Client, distDir string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/findings", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		views, err := listFindings(r.Context(), c)
		if err != nil {
			http.Error(w, "list findings", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(views)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if distDir == "" {
			http.Error(w, "ui not built", http.StatusServiceUnavailable)
			return
		}
		http.FileServer(http.Dir(distDir)).ServeHTTP(w, r)
	})
	return mux
}

func listFindings(ctx context.Context, c client.Client) ([]FindingView, error) {
	var l v1alpha1.FindingList
	if err := c.List(ctx, &l); err != nil {
		return nil, err
	}
	views := make([]FindingView, 0, len(l.Items))
	for _, f := range l.Items {
		views = append(views, FindingView{
			ID:       f.Name,
			Severity: strings.ToLower(string(f.Spec.Severity)),
			State:    strings.ToLower(string(f.Spec.State)),
			Kind:     f.Spec.TargetRef.Kind,
			Target:   f.Spec.TargetRef.Namespace + "/" + f.Spec.TargetRef.Name,
			// Defense in depth: sanitize again at the serving edge.
			LiveYaml:    evidence.Sanitize(f.Spec.PrePatchManifest),
			DiffRemoved: []string{},
			DiffAdded:   splitLines(f.Spec.ProposedPatch),
		})
	}
	return views, nil
}

func splitLines(s string) []string {
	if s == "" {
		return []string{}
	}
	return strings.Split(strings.TrimSuffix(s, "\n"), "\n")
}
