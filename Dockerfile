# syntax=docker/dockerfile:1

# ---- Build stage ----
FROM golang:1.26-alpine AS builder

RUN apk add --no-cache ca-certificates git

WORKDIR /src

# Dependency layer: copy manifests first so module downloads are cached
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Source
COPY . .

ARG VERSION=dev
ARG COMMIT=none

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux \
    go build -trimpath \
      -ldflags "-s -w -X tael.io/cli/cmd.version=${VERSION} -X tael.io/cli/cmd.commit=${COMMIT}" \
      -o /out/tael .

# ---- Runtime stage ----
FROM alpine:3.21

RUN apk add --no-cache ca-certificates \
 && addgroup -g 10001 -S tael \
 && adduser -u 10001 -S -G tael -h /home/tael tael

COPY --from=builder /out/tael /usr/local/bin/tael

USER 10001:10001
WORKDIR /home/tael

# The CLI does not bind a listener; exposed to match the manifest's container port.
EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/tael"]
CMD ["--help"]
