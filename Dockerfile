# The console is built first and baked into the Go binary with go:embed, so
# the running addon is ONE static binary in a distroless image: nothing to
# serve the console with, nothing to configure, one process.
# Both build stages run on the BUILD platform and cross-compile: the console
# build is platform-independent and Go cross-compiles for free, so a
# multi-arch image never runs npm or the Go toolchain under emulation (which
# turned a two-minute build into a forty-minute one).
FROM --platform=$BUILDPLATFORM node:20-alpine AS web
WORKDIR /web
COPY web/package.json web/package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY web/ ./
RUN npm run build          # -> /internal/api/web/embed

FROM --platform=$BUILDPLATFORM golang:1.24 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /internal/api/web/embed ./internal/api/web/embed
ARG VERSION=dev
ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /sample-addon .

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /sample-addon /sample-addon
EXPOSE 8080
ENTRYPOINT ["/sample-addon"]
