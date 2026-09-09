# The console is built first and baked into the Go binary with go:embed, so
# the running addon is ONE static binary in a distroless image: nothing to
# serve the console with, nothing to configure, one process.
FROM node:20-alpine AS web
WORKDIR /web
COPY web/package.json web/package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY web/ ./
RUN npm run build          # -> /internal/api/web/embed

FROM golang:1.24 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /internal/api/web/embed ./internal/api/web/embed
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -ldflags "-s -w -X main.version=${VERSION}" -o /sample-addon .

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /sample-addon /sample-addon
EXPOSE 8080
ENTRYPOINT ["/sample-addon"]
