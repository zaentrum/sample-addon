package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

// The drift-killer: every command the descriptor declares must be a real
// route with the declared method. A CLI surface describing an unshipped API is
// the documentation bug the capability design exists to prevent, and a test is
// the only place cheap enough to stop it forever.
func TestCapabilityCommandsAreRealRoutes(t *testing.T) {
	s := New(Config{AddonKey: "sample"})
	r := s.Handler().(chi.Router)
	routes := map[string]bool{}
	_ = chi.Walk(r, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		routes[method+" "+strings.TrimSuffix(route, "/")] = true
		return nil
	})
	doc := s.capabilityDoc()
	for _, c := range doc.Commands {
		if !routes[c.Method+" "+c.Path] {
			t.Errorf("descriptor declares %s %s but the router does not serve it", c.Method, c.Path)
		}
	}
	for _, c := range doc.Checks {
		if !routes["GET "+c.Path] {
			t.Errorf("descriptor declares check %q but GET %s is not routed", c.Name, c.Path)
		}
	}
}

func TestPublicSurface(t *testing.T) {
	s := New(Config{AddonKey: "sample", Version: "test"})
	h := s.Handler()
	for _, p := range []string{"/healthz", "/.well-known/zaentrum-capability.json", "/api/health/system", "/api/hello"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", p, nil))
		if rec.Code != 200 {
			t.Errorf("%s must be public and healthy, got %d", p, rec.Code)
		}
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/.well-known/zaentrum-capability.json", nil))
	var d struct {
		Service string `json:"service"`
		Kind    string `json:"kind"`
	}
	_ = json.NewDecoder(rec.Body).Decode(&d)
	if d.Service != "sample" || d.Kind != "addon" {
		t.Fatalf("descriptor must identify itself: %+v", d)
	}
	// The authenticated endpoint refuses anonymous callers with 401 — the
	// status zae maps to "forbidden", not "not offered".
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("POST", "/api/echo", strings.NewReader(`{"a":1}`)))
	if rec.Code != 401 {
		t.Fatalf("anonymous POST /api/echo must be 401, got %d", rec.Code)
	}
}

// The manifest's ui section is what the platform materialises on install. It
// must be self-consistent: a console that is declared must be baked into the
// binary, every slot row must have somewhere to go and something to say, and
// URLs must be portal-relative so the platform can absolutise them.
func TestManifestUIIsInstallable(t *testing.T) {
	s := New(Config{AddonKey: "sample"})
	ui := s.capabilityDoc().UI
	if ui == nil {
		t.Fatal("the reference addon declares a ui section")
	}
	if ui.App.Title == "" || ui.App.Icon == "" {
		t.Errorf("app needs a title and an icon: %+v", ui.App)
	}
	if ui.Console {
		rec := httptest.NewRecorder()
		s.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/embed/assets/remoteEntry.js", nil))
		if rec.Code != 200 {
			t.Errorf("console:true but /embed/assets/remoteEntry.js answers %d — build web/ first", rec.Code)
		}
	}
	if len(ui.Slots) == 0 {
		t.Fatal("the reference addon contributes at least one slot row")
	}
	for _, sl := range ui.Slots {
		if sl.Slot == "" || sl.Label == "" || sl.Key == "" {
			t.Errorf("slot row needs key, slot and label: %+v", sl)
		}
		if !strings.HasPrefix(sl.URL, "/") {
			t.Errorf("slot url must be portal-relative (the platform absolutises it): %q", sl.URL)
		}
		if !strings.Contains(sl.URL, "/portal/app/sample") {
			t.Errorf("slot url should open this addon's own console: %q", sl.URL)
		}
	}
}
