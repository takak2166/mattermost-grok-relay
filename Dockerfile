# syntax=docker/dockerfile:1

FROM golang:1.27-alpine AS builder

ARG TARGETOS=linux
ARG TARGETARCH=amd64

WORKDIR /src

COPY go.mod ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath \
    -ldflags="-s -w -X github.com/takak2166/mattermost-grok-relay/internal/app.Version=${VERSION:-0.0.0-dev}" \
    -o /out/mattermost-grok-relay ./cmd/mattermost-grok-relay

FROM alpine:3.21

RUN apk add --no-cache ca-certificates

WORKDIR /app
COPY --from=builder /out/mattermost-grok-relay /app/mattermost-grok-relay

USER nobody
ENTRYPOINT ["/app/mattermost-grok-relay"]
CMD ["-version"]
