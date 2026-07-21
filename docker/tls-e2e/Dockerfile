FROM golang:1.26-bookworm AS builder

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 go build -tags postgresql -o /deltascope-server ./cmd/deltascope-server

FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /deltascope-server /usr/local/bin/deltascope-server

EXPOSE 18932

ENTRYPOINT ["deltascope-server"]
CMD ["-listen", "0.0.0.0:18932", "-runtime-config", "/etc/deltascope/runtime.yaml"]
