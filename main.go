// Command sample-addon is the reference addon for the zaentrum platform.
//
// It is deliberately the smallest thing that plugs into every seam the
// platform exposes, so that someone writing a real addon can copy it:
//
//   - a UI slot contribution — on install it registers one button in chino's
//     "search.empty" slot, using its own service account (internal/register);
//   - a hosted console — a federated React module the portal mounts in-page
//     (web/, served from this binary at /embed/);
//   - a CLI capability descriptor — so `zae sample hello` exists on any
//     instance running this addon, and vanishes when it is removed
//     (internal/api/capability.go);
//   - two API endpoints reached through the portal's proxy: one public, one
//     that validates the caller's bearer against the instance's issuer.
//
// Everything it needs arrives as environment (see config); nothing about the
// instance is compiled in. Uninstalling it is subtraction: delete the
// workload and its one registry row, and the core shows no trace.
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
	"github.com/zaentrum/sample-addon/internal/register"
)

// version is stamped by the image build (-ldflags "-X main.version=…").
var version = "dev"

func main() {
	cfg := api.ConfigFromEnv()
	cfg.Version = version

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	srv := api.New(cfg)

	// Self-registration runs in the background and never blocks serving: an
	// addon that cannot register (portal down, identity not yet provisioned)
	// must still come up, report WHY in its health check, and keep retrying.
	// That is what "installing the addon brings its UI" means in practice.
	if reg := register.New(register.Config{
		TokenURL: cfg.TokenURL, ClientID: cfg.ClientID, ClientSecret: cfg.ClientSecret,
		PortalURL: cfg.PortalURL, PublicBase: cfg.PublicBase, AddonKey: cfg.AddonKey,
	}); reg != nil {
		go reg.Loop(ctx, srv.RecordRegistration)
	} else {
		srv.RecordRegistration(register.Status{Skipped: "no addon identity configured (CLIENT_ID/CLIENT_SECRET/PORTAL_URL)"})
	}

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
