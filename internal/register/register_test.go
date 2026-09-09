package register

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Registration = mint a client-credentials token, upsert one row. The row must
// carry the addon key (so uninstall is one deletion) and the public console URL
// with {q} for the query.
func TestOnceRegistersTheRow(t *testing.T) {
	var gotAuth, gotBody string
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		u, p, _ := r.BasicAuth()
		if u != "sample-svc" || p != "s3cret" {
			w.WriteHeader(401)
			return
		}
		fmt.Fprint(w, `{"access_token":"tok123"}`)
	})
	mux.HandleFunc("/api/portal/extensions", func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		b := make([]byte, 4096)
		n, _ := r.Body.Read(b)
		gotBody = string(b[:n])
		w.WriteHeader(200)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	reg := New(Config{TokenURL: srv.URL + "/token", ClientID: "sample-svc", ClientSecret: "s3cret",
		PortalURL: srv.URL, PublicBase: "https://media.example.org", AddonKey: "sample"})
	if err := reg.Once(context.Background()); err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer tok123" {
		t.Fatalf("service token not sent: %q", gotAuth)
	}
	for _, want := range []string{`"addon":"sample"`, `"slot":"search.empty"`, `"url":"https://media.example.org/portal/app/sample?q={q}"`} {
		if !strings.Contains(gotBody, want) {
			t.Errorf("row missing %s: %s", want, gotBody)
		}
	}
}

// The failure every stock install hits until the platform ships the addon role
// must be named, not reported as a bare status code.
func TestOnceExplainsMissingRole(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `{"access_token":"t"}`) })
	mux.HandleFunc("/api/portal/extensions", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(403) })
	srv := httptest.NewServer(mux)
	defer srv.Close()
	reg := New(Config{TokenURL: srv.URL + "/token", ClientID: "a", ClientSecret: "b", PortalURL: srv.URL, AddonKey: "sample"})
	err := reg.Once(context.Background())
	if err == nil || !strings.Contains(err.Error(), "addon role") {
		t.Fatalf("403 must be explained as the missing addon role, got %v", err)
	}
}

func TestNewIsNilWithoutIdentity(t *testing.T) {
	if New(Config{PortalURL: "http://portal-api"}) != nil {
		t.Fatal("no client credentials must mean no registrar, not a crash loop")
	}
}
