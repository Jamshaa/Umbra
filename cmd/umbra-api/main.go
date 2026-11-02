// Command umbra-api serves the read-only Umbra UI backend on localhost.
//
// GET /api/findings lists Finding CRs from the kind-umbra cluster;
// / serves the vite bundle from ui/dist. No route writes anything.
package main

import (
	"net/http"
	"os"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/Jamshaa/Umbra/api/v1alpha1"
	umbraapi "github.com/Jamshaa/Umbra/internal/api"
)

func main() {
	scheme := runtime.NewScheme()
	if err := v1alpha1.AddToScheme(scheme); err != nil {
		ctrl.Log.Error(err, "add Umbra scheme")
		os.Exit(1)
	}
	c, err := client.New(ctrl.GetConfigOrDie(), client.Options{Scheme: scheme})
	if err != nil {
		ctrl.Log.Error(err, "create client")
		os.Exit(1)
	}
	dist := os.Getenv("UMBRA_DIST")
	if dist == "" {
		dist = "ui/dist"
	}
	addr := "127.0.0.1:8082"
	ctrl.Log.Info("serving umbra-api", "addr", addr, "dist", dist)
	srv := &http.Server{
		Addr:              addr,
		Handler:           umbraapi.NewHandler(c, dist),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	if err := srv.ListenAndServe(); err != nil {
		ctrl.Log.Error(err, "serve")
		os.Exit(1)
	}
}
