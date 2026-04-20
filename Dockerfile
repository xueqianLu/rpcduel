# Multi-stage Dockerfile for local builds. Releases use Dockerfile.goreleaser.
FROM golang:1.23-alpine AS builder

WORKDIR /src
RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 go build \
    -trimpath \
    -ldflags "-s -w -X main.version=${VERSION}" \
    -o /out/rpcduel .

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /out/rpcduel /usr/local/bin/rpcduel
USER nonroot:nonroot
ENTRYPOINT ["/usr/local/bin/rpcduel"]
