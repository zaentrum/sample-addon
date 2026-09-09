// Package register is self-registration: on install, the addon puts its own
// button into a product slot by writing one row to the portal's extension
// registry, authenticating as itself.
//
// This is the seam the platform docs call "installing the addon brings its
// UI". The row is keyed by `addon`, so uninstalling is one deletion and the
// core shows no trace afterwards. The addon authenticates with the OAuth2
// client-credentials grant against the instance's issuer: a confidential
// client whose service account carries the instance's addon role.
package register

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Config struct {
	TokenURL     string
	ClientID     string
	ClientSecret string
	PortalURL    string
	PublicBase   string
	AddonKey     string
	// HTTP is injectable for tests; nil means a 15 s client.
	HTTP *http.Client
}

// Status is what the health check reports. Detail() is written for the person
// reading it, because "registration failed" alone is where diagnosis starts,
// not where it ends.
type Status struct {
	Registered bool
	Attempts   int
	LastError  string
	At         time.Time
	Skipped    string // non-empty when registration was not attempted at all
}

func (s Status) Detail() string {
	switch {
	case s.Skipped != "":
		return "skipped: " + s.Skipped
	case s.Registered:
		return fmt.Sprintf("registered at %s after %d attempt(s)", s.At.Format(time.RFC3339), s.Attempts)
	case s.LastError != "":
		return fmt.Sprintf("not registered after %d attempt(s): %s", s.Attempts, s.LastError)
	default:
		return "not attempted yet"
	}
}

type Registrar struct{ cfg Config }

// New returns nil when the identity is not configured, so the caller can serve
// without registration rather than crash-looping on a missing secret.
func New(cfg Config) *Registrar {
	if cfg.ClientID == "" || cfg.ClientSecret == "" || cfg.PortalURL == "" || cfg.TokenURL == "" {
		return nil
	}
	if cfg.HTTP == nil {
		cfg.HTTP = &http.Client{Timeout: 15 * time.Second}
	}
	return &Registrar{cfg: cfg}
}

// Loop retries registration with backoff until it succeeds or ctx ends,
// reporting each outcome. It never gives up: the portal or the identity
// provider may come up long after the addon does.
func (r *Registrar) Loop(ctx context.Context, report func(Status)) {
	st := Status{}
	delay := 5 * time.Second
	for {
		st.Attempts++
		if err := r.Once(ctx); err != nil {
			st.LastError = err.Error()
			report(st)
			log.Printf("register: attempt %d failed: %v (retrying in %s)", st.Attempts, err, delay)
		} else {
			st.Registered, st.LastError, st.At = true, "", time.Now()
			report(st)
			log.Printf("register: slot row %q registered", r.row().Key)
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
		if delay < 2*time.Minute {
			delay *= 2
		}
	}
}

// Row is the ui_extensions contribution. One button, in one slot, pointing at
// this addon's own console with the user's search carried along as {q}.
type Row struct {
	Key     string `json:"key"`
	Addon   string `json:"addon"`
	Slot    string `json:"slot"`
	Kind    string `json:"kind"`
	Label   string `json:"label"`
	Icon    string `json:"icon"`
	URL     string `json:"url"`
	Ord     int    `json:"ord"`
	Enabled bool   `json:"enabled"`
}

func (r *Registrar) row() Row {
	return Row{
		Key:     r.cfg.AddonKey + ".search-hint",
		Addon:   r.cfg.AddonKey,
		Slot:    "search.empty",
		Kind:    "link",
		Label:   "Try the sample addon",
		Icon:    "puzzle",
		URL:     strings.TrimRight(r.cfg.PublicBase, "/") + "/portal/app/" + r.cfg.AddonKey + "?q={q}",
		Ord:     90,
		Enabled: true,
	}
}

// Once performs a single registration: mint a service token, upsert the row.
func (r *Registrar) Once(ctx context.Context) error {
	tok, err := r.token(ctx)
	if err != nil {
		return fmt.Errorf("mint service token: %w", err)
	}
	body, _ := json.Marshal(r.row())
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost,
		strings.TrimRight(r.cfg.PortalURL, "/")+"/api/portal/extensions", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	resp, err := r.cfg.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("portal-api unreachable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		// The one failure every stock install hits until the platform ships the
		// addon role: say exactly what is missing.
		return fmt.Errorf("portal-api answered %d: the service account lacks the addon role (realm role zaentrum-addon, or whatever PORTAL_ADDON_ROLE names)", resp.StatusCode)
	}
	if resp.StatusCode/100 != 2 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("portal-api answered %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return nil
}

func (r *Registrar) token(ctx context.Context) (string, error) {
	form := url.Values{"grant_type": {"client_credentials"}}
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, r.cfg.TokenURL, strings.NewReader(form.Encode()))
	req.SetBasicAuth(r.cfg.ClientID, r.cfg.ClientSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := r.cfg.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", fmt.Errorf("token endpoint answered %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	var t struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&t); err != nil || t.AccessToken == "" {
		return "", fmt.Errorf("token endpoint returned no access_token")
	}
	return t.AccessToken, nil
}
