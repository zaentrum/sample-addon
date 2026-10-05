# sample-addon — the reference addon for zaentrum

The smallest thing that plugs into every seam the
[zaentrum](https://github.com/zaentrum/zaentrum) platform exposes. It exists
to be copied: a real addon has more behaviour, but not more *kinds* of
integration than this one.

| Seam | What this addon does | Where |
|---|---|---|
| **UI slot** | Declares one button for chino's `search.empty` slot; the platform creates the row when an admin installs the addon | `internal/api/capability.go` (`ui.slots`) |
| **Hosted console** | A React module the portal mounts in-page, served by this binary; declared with `ui.console` so the platform places a tile | `web/`, `internal/api/api.go` (`/embed/`) |
| **CLI capability** | Declares two commands and a check, so `zae sample hello` exists wherever it runs | `internal/api/capability.go` |
| **Its own API** | One endpoint that asks for no role; one that validates the user's bearer against the instance's issuer. Through the portal's proxy both need a signed-in user | `internal/api/api.go` |
| **Notices** | The console's **Send Me a Notice** tells the signed-in person something; the bell in the portal's header shows it, and so do the apps | `web/src/Console.tsx` (`sendNotice`) |

One document — the capability manifest — declares all of it. The addon holds
no credentials: an admin adds it in the portal's settings by its in-cluster
address, and the platform pulls the manifest and creates what it declares. Its
one write to the platform is a notice its console posts with the signed-in
person's own bearer, which reaches that person and nobody else.

## Notices

`sendNotice` in the console posts `{addon, title, body, link}` to the portal's
`POST /api/portal/me/notices` with the bearer the shell handed the console: a
notice to the person whose bearer it is, from this addon, with a link back to
the console. The portal's API is what comes before `/apps/` in `apiBase`, and
the addon's key what follows it. The title is at most 80 characters, the body
at most 280, both plain text; the link is a path on the instance.

That is the one way to post a notice without a credential, and why it reaches
nobody else: the body names no person. An addon that tells someone something
later — when their title is ready — posts with its service account instead,
`POST /api/portal/notices {sub, title, body, link?, itemId?}`, where `sub` is
the person's token subject, which its API reads from the bearer it validates
when they call it. The service account is a confidential client named after
the addon, with the `zaentrum-addon` role; the platform docs, *Extend it →
Notices* and *Addon identity*, say how to make one.

The platform-side contracts are documented canonically under
**[Extend it](https://github.com/zaentrum/zaentrum/wiki/extending)** in the
zaentrum docs. This repo shows one honest implementation of each.

## Try it against a running instance

```
$ zae discover --url https://<instance>
sample (addon)
  zae sample hello               say hello (no role needed; the portal asks for a sign-in)
  zae sample echo                echo a JSON body back (requires a signed-in user)
  check: system

$ zae login --url https://<instance>
$ zae sample hello --url https://<instance>
{"hello":"from the sample addon","addon":"sample","version":"…"}
```

Every addon command goes through the portal's proxy, which turns away a
request without a signed-in user's bearer (401, `zae` exit 5). The addon's
console bundle (`/embed/`) and its descriptor (`/.well-known/`) are the only
paths the proxy serves without one.

## What to copy, and what to change

- **`web/vite.embed.config.ts`** *is* the console hosting contract. Copy it
  verbatim; change `name`. Your module must export a **named** `Console`
  taking `{ apiBase, token, onUnauthorized }` (see `web/src/Console.tsx`).
  React and react-dom are shared with the shell at `^18.3.0`.
- **`internal/api/capability.go`** — declare only real routes;
  `capability_test.go` walks the router and fails the build on drift. Keep
  that test. Ten curated commands beat eighty generated ones.
- **`ui` in the manifest** — the app's title and icon, whether it has a
  console, and its slot rows. Slot URLs are portal-relative; the platform
  absolutises them on install. `capability_test.go` checks the section is
  installable (a declared console is actually in the binary, every row has a
  slot and a label).
- **`deploy/`** — the install unit. One value is yours (`OIDC_ISSUER`);
  everything else is generic. No Secret.

The console uses plain CSS on purpose: a reference should not hand you a
design-system dependency. [acquire](https://github.com/laedeli/acquire) is
the full-size example with the platform's design system.

## Develop

```sh
go run .                          # API + descriptor on :8080 (no console yet)
cd web && npm install && npm run dev   # console standalone, proxies /api → :8080
cd web && npm run build && go build .  # bake the console into the binary
```

`go test ./...` covers route-reality for the descriptor, the public surface,
and that the manifest's `ui` section is installable.

## Honest status

| | |
|---|---|
| Slot button, console, descriptor, open + authenticated endpoints | ✅ |
| Install from the portal's settings (app, tile, slot row created by the platform) | ✅ — one action, no identity, removable by key |
| A notice to the signed-in person, from the console | ✅ — with their own bearer, to them only |
| A notice to someone else, or later | not here — it takes a service account, which this addon does not hold |
| Events on the bus | none — this addon emits nothing, and its descriptor says so |

## License

[MPL-2.0](./LICENSE), like the platform.
