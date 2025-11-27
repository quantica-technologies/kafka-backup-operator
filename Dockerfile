FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git make ca-certificates tzdata

WORKDIR /workspace

# Copy go.mod and go.sum first for better caching
COPY go.mod go.sum ./

# Verify go.mod
RUN go mod verify || true

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH:-amd64} \
    go build -a -installsuffix cgo -ldflags="-w -s" \
    -o manager ./cmd/operator

FROM gcr.io/distroless/static:nonroot

WORKDIR /

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /workspace/manager .

USER 65532:65532

ENTRYPOINT ["/manager"]