// Command demo-server is the throwaway workload for live Umbra demos.
//
// It only keeps TCP :8080 open (demo probes are tcpSocket) and answers
// / with a static line. No dependencies beyond the standard library,
// no state, no network calls.
package main

import (
	"fmt"
	"net/http"
	"os"
	"time"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintln(w, "umbra demo-app: ok")
	})
	srv := &http.Server{
		Addr:              ":8080",
		Handler:           nil,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	if err := srv.ListenAndServe(); err != nil {
		fmt.Fprintln(os.Stderr, "demo-server:", err)
		os.Exit(1)
	}
}
