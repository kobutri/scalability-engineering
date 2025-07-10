# syntax=docker/dockerfile:1
FROM golang:1.24-alpine AS builder

# Install templ
RUN go install github.com/a-h/templ/cmd/templ@latest

WORKDIR /workspace

# Copy go.mod and go.sum first for better caching
COPY shared/go.mod shared/go.sum ./shared/
COPY client/go.mod client/go.sum ./client/

# Download dependencies with cache mount
WORKDIR /workspace/client
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Copy source code
COPY shared/ ../shared/
COPY client/ .

# Generate templ templates with cache mount
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    templ generate

# Build with cache mounts for faster builds
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -trimpath -o main .

# Final stage
FROM alpine:latest
WORKDIR /root/

# Add curl for health checks
RUN apk add --no-cache curl

COPY --from=builder /workspace/client/main .
EXPOSE 9090
CMD ["./main"]