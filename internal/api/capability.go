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

type capability struct {
	Service  string       `json:"service"`
	Kind     string       `json:"kind"`
	Version  string       `json:"version,omitempty"`
	Commands []capCommand `json:"commands"`
	Checks   []capCheck   `json:"checks"`
	Topics   []string     `json:"topics"`
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
	}
}

func (s *Server) capability(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.capabilityDoc())
}
