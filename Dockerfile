# syntax=docker/dockerfile:1

FROM golang:1.24-alpine AS builder

WORKDIR /src

COPY go.mod ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/mattermost-grok-relay ./cmd/mattermost-grok-relay

FROM alpine:3.21

RUN apk add --no-cache ca-certificates

WORKDIR /app
COPY --from=builder /out/mattermost-grok-relay /app/mattermost-grok-relay

USER nobody
ENTRYPOINT ["/app/mattermost-grok-relay"]
CMD ["-version"]
