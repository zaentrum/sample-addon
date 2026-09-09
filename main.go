// Command sample-addon is the reference addon for the zaentrum platform.
//
// It is deliberately the smallest thing that plugs into every seam the
// platform exposes, so that someone writing a real addon can copy it:
//
//   - a UI slot contribution — one button in chino's "search.empty" slot,
//     declared in the capability descriptor; the platform creates the row
//     when an admin installs the addon (internal/api/capability.go);
//   - a hosted console — a federated React module the portal mounts in-page
//     (web/, served from this binary at /embed/), likewise declared;
//   - a CLI capability descriptor — so `zae sample hello` exists on any
//     instance running this addon, and vanishes when it is removed
//     (internal/api/capability.go);
//   - two API endpoints reached through the portal's proxy: one public, one
//     that validates the caller's bearer against the instance's issuer.
//
// Everything it needs arrives as environment (see config); nothing about the
// instance is compiled in, and it holds no credentials of its own. Installing
// is pull: an admin adds it in the portal's settings by its in-cluster
// address. Uninstalling is subtraction: remove it there, delete the workload,
// and the core shows no trace.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/zaentrum/sample-addon/internal/api"
)

// version is stamped by the image build (-ldflags "-X main.version=…").
var version = "dev"

func main() {
	cfg := api.ConfigFromEnv()
	cfg.Version = version

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	srv := api.New(cfg)

	httpSrv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(shutdown)
	}()
	log.Printf("sample-addon %s listening on :%s", version, cfg.Port)
	if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
	_ = os.Stdout.Sync()
}
