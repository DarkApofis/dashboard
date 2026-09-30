# Build stage: the toolchain never ships in the final image.
FROM golang:1.25-alpine AS build

WORKDIR /src

# Dependencies first, so editing source does not invalidate the module cache.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=docker
# CGO off gives a static binary, which is what lets the runtime image be scratch-thin.
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -o /out/api ./cmd/api

# Runtime stage.
FROM alpine:3.21

# ca-certificates is not needed today, but any outbound HTTPS call later would fail
# silently without it. wget is already in busybox and backs the healthcheck.
RUN apk add --no-cache ca-certificates tzdata \
    && adduser -D -u 10001 app

COPY --from=build /out/api /usr/local/bin/api

USER app
EXPOSE 8080

# The process reads everything from the environment; see .env.example.
ENTRYPOINT ["/usr/local/bin/api"]
