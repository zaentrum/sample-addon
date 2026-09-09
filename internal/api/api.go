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

	"github.com/zaentrum/sample-addon/internal/register"
)

// Config is everything the addon learns about its instance. All of it comes
// from the environment; none of it is compiled in.
type Config struct {
	Port string
	// OIDCIssuer is the instance's issuer (advertised by /api/config on the
	// instance). The addon validates user bearers against it.
	OIDCIssuer string
	// Self-registration identity: a confidential client with a service account
	// carrying the instance's addon role. See internal/register.
	TokenURL     string
	ClientID     string
	ClientSecret string
	PortalURL    string // in-cluster portal-api, e.g. http://portal-api
	PublicBase   string // public origin, for the slot button's URL
	AddonKey     string // registry key: app key AND descriptor service name
	Version      string
}

func env(k, def string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return def
}

func ConfigFromEnv() Config {
	issuer := env("OIDC_ISSUER", "")
	return Config{
		Port:         env("PORT", "8080"),
		OIDCIssuer:   issuer,
		TokenURL:     env("OIDC_TOKEN_URL", strings.TrimRight(issuer, "/")+"/protocol/openid-connect/token"),
		ClientID:     env("CLIENT_ID", ""),
		ClientSecret: env("CLIENT_SECRET", ""),
		PortalURL:    env("PORTAL_URL", ""),
		PublicBase:   env("PUBLIC_BASE", ""),
		AddonKey:     env("ADDON_KEY", "sample"),
	}
}

//go:embed all:web
var webFS embed.FS

type Server struct {
	cfg      Config
	verifier *oidc.IDTokenVerifier // nil until the issuer is reachable
	verifyMu sync.Mutex

	regMu sync.Mutex
	reg   register.Status
}

func New(cfg Config) *Server { return &Server{cfg: cfg} }

// RecordRegistration is how the register loop reports; the health check
// surfaces it, so "installed but its button never appeared" is diagnosable
// from the outside instead of being a mystery.
func (s *Server) RecordRegistration(st register.Status) {
	s.regMu.Lock()
	defer s.regMu.Unlock()
	s.reg = st
}

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

// systemHealth is the addon's registered check for `zae doctor`: it reports
// whether self-registration succeeded, which is the one thing an operator
// cannot otherwise see without reading logs.
func (s *Server) systemHealth(w http.ResponseWriter, _ *http.Request) {
	s.regMu.Lock()
	st := s.reg
	s.regMu.Unlock()
	checks := []map[string]any{{
		"name":   "slot registration",
		"ok":     st.Registered,
		"detail": st.Detail(),
	}}
	status := "ok"
	if !st.Registered && st.Skipped == "" {
		status = "degraded"
	}
	writeJSON(w, 200, map[string]any{"status": status, "checks": checks, "version": s.cfg.Version})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
