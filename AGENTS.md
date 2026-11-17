# sample-addon

The reference addon of zaentrum: the smallest addon that uses every seam the platform
offers, made to be copied. One Go binary serves its capability manifest
(`/.well-known/zaentrum-capability.json`), which declares a button in chino's `search.empty`
slot, a console, two CLI commands and a check; its own API, `/api/hello` (no role) and
`/api/echo` (checks the caller's bearer against `OIDC_ISSUER`); and the console in `web/`, a
federated React module the portal mounts in-page from `/embed/`. An admin installs it in the
portal's settings by its in-cluster address; the portal's proxy and the `zae` CLI call it, and
its console posts notices to the portal with the signed-in person's own bearer.
How a change is proposed and what a pull request has to pass is in the
[guide to contributing](https://github.com/zaentrum/.github/blob/main/CONTRIBUTING.md);
what may be built is in [building from the specification](https://github.com/zaentrum/zaentrum/blob/main/AGENTS.md).

## Build

```sh
(cd web && npm ci && npm run build)   # the console, into internal/api/web/embed
go build .
docker build -t sample-addon .        # distroless, on 8080
```

The console comes first: `//go:embed all:web` in `internal/api/api.go` embeds
`internal/api/web/embed`, which the web build writes. A checkout without it builds, from the
placeholder `.keep` alone, but serves no console, and the tests that ask for one fail. It needs Go 1.24
(`go.mod`) and Node (the workflow and the image use 20). `image.yml` passes `VERSION` and
builds for linux/amd64 and linux/arm64.

## Test

As `.github/workflows/ci.yml` runs them, in its job `test`:

```sh
(cd web && npm ci --no-audit --no-fund && npm run build)
test -z "$(gofmt -l .)"
go vet ./...
go test ./...
docker build -t sample-addon:ci .
```

The tests, in `internal/api/capability_test.go`, need nothing beyond the console's build and
skip nothing: every command the manifest declares is a route with its method; the public
paths answer 200 and `/api/echo` refuses a caller without a bearer with 401; and the
manifest's `ui` section is installable (a declared console is in the binary, every slot row
has a key, a slot, a label and a portal-relative link to this addon's console).

## Run it

```sh
go run .                    # after the console's build: http://localhost:8080
(cd web && npm run dev)     # the console alone: http://localhost:5173, /api proxied to :8080
```

It reads `PORT` (default 8080), `ADDON_KEY` (default `sample`) and `OIDC_ISSUER`.
`/healthz`, the manifest, `/api/health/system`, `/api/hello` and `/embed/` need nothing;
without `OIDC_ISSUER` a bearer sent to `/api/echo` gets 503. Standalone the console has no
portal around it and no token: **call it** answers, **echo it** gets 401, and **Send Me a
Notice** says it works only inside the portal.

In a platform, such as the reference demo
([how it is deployed](https://github.com/zaentrum/zaentrum/blob/main/docs/reference-demo.md)),
the workload goes next to it with `kubectl apply -k deploy/` (set `OIDC_ISSUER` and the
namespace first) and an admin installs it by its address in the portal's settings:
[deploy/README.md](deploy/README.md). `zae discover --url <the platform's address>` then
lists `zae sample hello`.

## What the tests leave unchecked

- The console has no tests: `npm run build` type-checks and builds it, nothing runs it.
- No test mounts the console in the portal or holds `web/vite.embed.config.ts` against the
  portal's host; the install, the slot button, the tile and the notice are tried on a
  platform, by hand.
- `/api/echo` with a valid bearer: no test meets an issuer.
- `deploy/` is neither applied nor validated by the workflow.
- The image is built for one platform on a pull request, not started; the two-platform
  build runs only in `image.yml`, on main and on a release tag.

## Before a pull request

`ci.yml` fails on two checks of its own:

- `gofmt -l .` has to print nothing; `gofmt -w .` formats the code.
- Its neutrality gate searches `*.go`, `*.md`, `*.ts`, `*.tsx`, `*.yaml` and `*.yml` for the
  project's internal host names and the names of download tools; a match fails the job.
