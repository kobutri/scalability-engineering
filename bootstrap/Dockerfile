# syntax=docker/dockerfile:1
FROM golang:1.24-alpine AS builder

WORKDIR /workspace

# Copy go.mod and go.sum first for better caching
COPY shared/go.mod shared/go.sum ./shared/
COPY bootstrap/go.mod bootstrap/go.sum ./bootstrap/

# Download dependencies with cache mount
WORKDIR /workspace/bootstrap
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Copy source code
COPY shared/ ../shared/
COPY bootstrap/ .

# Build with cache mounts for faster builds
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main .

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

COPY --from=builder /workspace/bootstrap/main .

EXPOSE 8080

CMD ["./main"] 