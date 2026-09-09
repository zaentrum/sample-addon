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
| **Its own API** | One public endpoint; one that validates the user's bearer against the instance's issuer | `internal/api/api.go` |

One document — the capability manifest — declares all of it. The addon never
writes to the platform and holds no credentials: an admin adds it in the
portal's settings by its in-cluster address, and the platform pulls the
manifest and creates what it declares.

The platform-side contracts are documented canonically under
**[Extend it](https://github.com/zaentrum/zaentrum/wiki/extending)** in the
zaentrum docs. This repo shows one honest implementation of each.

## Try it against a running instance

```
$ zae discover --url https://<instance>
sample (addon)
  zae sample hello               say hello (public — exits 0 with no login)
  zae sample echo                echo a JSON body back (requires a signed-in user)
  check: system

$ zae sample hello --url https://<instance>
{"hello":"from the sample addon","addon":"sample","version":"…"}
```

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
| Slot button, console, descriptor, public + authenticated endpoints | ✅ |
| Install from the portal's settings (app, tile, slot row created by the platform) | ✅ — one action, no identity, removable by key |
| Events on the bus | none — this addon emits nothing, and its descriptor says so |

## License

[MPL-2.0](./LICENSE), like the platform.
