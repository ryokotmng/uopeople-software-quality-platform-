# Multi-stage build for the Software Quality Platform.
# The application uses only the Go standard library, so the final image
# is a single static binary on a minimal base — small and reproducible.

# --- Build stage ---
FROM golang:1.26 AS build
WORKDIR /src

# Module files first for layer caching (no external dependencies).
COPY go.mod ./
RUN go mod download

COPY . .
# CGO disabled -> a fully static binary that runs on a minimal image.
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/server ./cmd/server

# --- Runtime stage ---
FROM alpine:3.20
# Run as an unprivileged user with a writable data directory.
RUN adduser -D -u 10001 app && mkdir -p /data && chown app /data
USER app

COPY --from=build /out/server /usr/local/bin/server

# ADDR / DATA_DIR can be overridden at run time; PORT is also honored.
ENV ADDR=:8080 \
    DATA_DIR=/data
EXPOSE 8080

ENTRYPOINT ["server"]
