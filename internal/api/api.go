// Package api is the addon's HTTP surface. Three kinds of route, and the
// distinction is the lesson:
//
//   - public metadata the platform reads without credentials (/healthz, the
//     capability descriptor, the diagnostic check);
//   - the embedded console the portal fetches through its proxy (/embed/);
//   - the addon's own API, reached through the same proxy with the user's
//     bearer forwarded — which the addon validates itself against the
//     instance's issuer. The portal forwards tokens; it does not vouch for
//     them.
//
// There is no registration code anywhere in this addon. What it contributes
// to the UI is DECLARED in the capability descriptor (capability.go); the
// platform pulls that manifest when an admin installs the addon and creates
// the rows itself.
package api

import (
	"context"
	"embed"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// Config is everything the addon learns about its instance. All of it comes
// from the environment; none of it is compiled in — and there is deliberately
// little of it. An addon needs no identity of its own to be installed: the
// platform pulls its manifest and creates what it declares.
type Config struct {
	Port string
	// OIDCIssuer is the instance's issuer (advertised by /api/config on the
	// instance). The addon validates user bearers against it. Only the
	// authenticated endpoint needs it; everything else works without.
	OIDCIssuer string
	// AddonKey is the descriptor's service name, which the platform uses as
	// the app key when it installs the addon.
	AddonKey string
	Version  string
}

func env(k, def string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return def
}

func ConfigFromEnv() Config {
	return Config{
		Port:       env("PORT", "8080"),
		OIDCIssuer: env("OIDC_ISSUER", ""),
		AddonKey:   env("ADDON_KEY", "sample"),
	}
}

//go:embed all:web
var webFS embed.FS

type Server struct {
	cfg      Config
	verifier *oidc.IDTokenVerifier // nil until the issuer is reachable
	verifyMu sync.Mutex
}

func New(cfg Config) *Server { return &Server{cfg: cfg} }

func (s *Server) Handler() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RealIP, middleware.Recoverer)

	// Public metadata. Unauthenticated on purpose: the platform's discovery
	// aggregator and the CLI's doctor read these without credentials, and a
	// health surface that needs working auth is useless when auth is what
	// broke.
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, 200, map[string]string{"status": "ok"}) })
	r.Get("/.well-known/zaentrum-capability.json", s.capability)
	r.Get("/api/health/system", s.systemHealth)

	// The console, served from the binary. The portal fetches
	// /embed/assets/remoteEntry.js and /embed/assets/console.css through its
	// proxy; relative asset URLs keep it working wherever it is mounted.
	sub, _ := fs.Sub(webFS, "web/embed")
	r.Handle("/embed/*", http.StripPrefix("/embed/", http.FileServer(http.FS(sub))))

	// The addon's API. One public endpoint so the dynamic CLI has something
	// that exits 0 with no login; one that requires a valid user bearer, so
	// the reference shows how an addon validates the caller.
	r.Get("/api/hello", s.hello)
	r.Post("/api/echo", s.requireUser(s.echo))
	return r
}

func (s *Server) hello(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{
		"hello":   "from the sample addon",
		"addon":   s.cfg.AddonKey,
		"version": s.cfg.Version,
		"note":    "this endpoint is public; POST /api/echo requires a signed-in user",
	})
}

func (s *Server) echo(w http.ResponseWriter, r *http.Request) {
	var body map[string]any
	_ = json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&body)
	if body == nil {
		body = map[string]any{}
	}
	writeJSON(w, 200, map[string]any{
		"echo":    body,
		"subject": r.Context().Value(subjectKey{}),
	})
}

type subjectKey struct{}

// requireUser validates the forwarded bearer against the instance's issuer.
// The verifier is built lazily and retried: an addon must not refuse to boot
// because the identity provider was slow to come up alongside it.
func (s *Server) requireUser(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		raw := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if raw == "" || raw == r.Header.Get("Authorization") {
			http.Error(w, "a signed-in user is required", http.StatusUnauthorized)
			return
		}
		v, err := s.getVerifier(r.Context())
		if err != nil {
			http.Error(w, "identity provider unavailable: "+err.Error(), http.StatusServiceUnavailable)
			return
		}
		tok, err := v.Verify(r.Context(), raw)
		if err != nil {
			http.Error(w, "invalid token: "+err.Error(), http.StatusUnauthorized)
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), subjectKey{}, tok.Subject)))
	}
}

func (s *Server) getVerifier(ctx context.Context) (*oidc.IDTokenVerifier, error) {
	s.verifyMu.Lock()
	defer s.verifyMu.Unlock()
	if s.verifier != nil {
		return s.verifier, nil
	}
	if s.cfg.OIDCIssuer == "" {
		return nil, errNoIssuer
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	p, err := oidc.NewProvider(ctx, s.cfg.OIDCIssuer)
	if err != nil {
		return nil, err
	}
	// The instance issues one unified client for its products; an addon is a
	// resource server behind the portal proxy, so it does not pin an audience.
	s.verifier = p.Verifier(&oidc.Config{SkipClientIDCheck: true})
	log.Printf("oidc: verifying bearers against %s", s.cfg.OIDCIssuer)
	return s.verifier, nil
}

type noIssuer struct{}

func (noIssuer) Error() string { return "OIDC_ISSUER is not configured" }

var errNoIssuer = noIssuer{}

// systemHealth is the check the addon declares for `zae doctor`. It reports
// the two things an operator cannot see from outside: whether the console is
// actually baked into this binary, and whether the authenticated endpoint can
// work at all (an issuer is configured). Neither failing stops the addon from
// serving — that is the point of reporting them.
func (s *Server) systemHealth(w http.ResponseWriter, _ *http.Request) {
	_, consoleErr := fs.Stat(webFS, "web/embed/assets/remoteEntry.js")
	checks := []map[string]any{
		{
			"name":   "console",
			"ok":     consoleErr == nil,
			"detail": consoleDetail(consoleErr),
		},
		{
			"name":   "issuer",
			"ok":     s.cfg.OIDCIssuer != "",
			"detail": issuerDetail(s.cfg.OIDCIssuer),
		},
	}
	status := "ok"
	for _, c := range checks {
		if ok, _ := c["ok"].(bool); !ok {
			status = "degraded"
		}
	}
	writeJSON(w, 200, map[string]any{"status": status, "checks": checks, "version": s.cfg.Version})
}

func consoleDetail(err error) string {
	if err != nil {
		return "no console in this binary — build web/ before go build, or the portal tile opens an empty page"
	}
	return "federated console embedded at /embed/"
}

func issuerDetail(issuer string) string {
	if issuer == "" {
		return "OIDC_ISSUER unset — POST /api/echo will answer 503; public endpoints unaffected"
	}
	return "verifying user bearers against " + issuer
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
