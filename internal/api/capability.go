package api

import "net/http"

// The capability descriptor: how this addon extends the zae CLI without zae
// ever compiling in its name. The platform's portal aggregates this document
// from every running service; zae renders what it finds. Install the addon
// and `zae sample hello` exists on that instance; remove it and it is gone.
//
// Contract: capability schema v1 — see the platform docs, "The CLI contract".
// Two self-imposed rules a real addon should copy:
//   - every command here must be a REAL route: capability_test.go walks the
//     router and fails the build on drift;
//   - paths are service-relative; the caller reaches them through whatever
//     front door it has (the portal's proxy, from outside).

type capCommand struct {
	Name    string `json:"name"`
	Summary string `json:"summary"`
	Method  string `json:"method"`
	Path    string `json:"path"`
	Role    string `json:"role,omitempty"`
}

type capCheck struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// capUI is what the addon contributes to the portal and the product apps.
// The platform reads it when an admin installs the addon in settings and
// creates the app, the tile and the slot rows itself — the addon never
// writes to a registry and needs no identity to appear.
type capUI struct {
	App     capApp    `json:"app"`
	Console bool      `json:"console"`
	Slots   []capSlot `json:"slots"`
}

type capApp struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
}

// capSlot is one contribution to a product slot. URLs are relative to the
// portal: the addon does not know where it is installed; the platform
// absolutises them at install time.
type capSlot struct {
	Key   string `json:"key"`
	Slot  string `json:"slot"`
	Kind  string `json:"kind"`
	Label string `json:"label"`
	Icon  string `json:"icon"`
	URL   string `json:"url"`
	Ord   int    `json:"ord"`
}

type capability struct {
	Service  string       `json:"service"`
	Kind     string       `json:"kind"`
	Version  string       `json:"version,omitempty"`
	Commands []capCommand `json:"commands"`
	Checks   []capCheck   `json:"checks"`
	Topics   []string     `json:"topics"`
	UI       *capUI       `json:"ui,omitempty"`
}

func (s *Server) capabilityDoc() capability {
	return capability{
		// The service name doubles as the portal app key the proxy uses
		// (/api/portal/apps/<key>/…), so the two must agree — or set
		// "proxyKey" in the descriptor when they cannot.
		Service: s.cfg.AddonKey,
		Kind:    "addon",
		Version: s.cfg.Version,
		Commands: []capCommand{
			{Name: "hello", Summary: "say hello (public — exits 0 with no login)", Method: "GET", Path: "/api/hello"},
			{Name: "echo", Summary: "echo a JSON body back (requires a signed-in user)", Method: "POST", Path: "/api/echo", Role: "user"},
		},
		Checks: []capCheck{
			{Name: "system", Path: "/api/health/system"},
		},
		// No topics: this addon emits no events, and a descriptor must not
		// claim what the service does not do.
		Topics: []string{},
		UI: &capUI{
			App: capApp{
				Title:       "Sample addon",
				Description: "the reference addon — one button, one console, two commands",
				Icon:        "puzzle",
			},
			// The console is served from this binary at /embed/; the platform
			// hosts it at /portal/app/<service> and places a tile for it.
			Console: true,
			Slots: []capSlot{{
				Key:   "search-hint",
				Slot:  "search.empty",
				Kind:  "link",
				Label: "Try the sample addon",
				Icon:  "puzzle",
				URL:   "/portal/app/" + s.cfg.AddonKey + "?q={q}",
				Ord:   90,
			}},
		},
	}
}

func (s *Server) capability(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.capabilityDoc())
}
