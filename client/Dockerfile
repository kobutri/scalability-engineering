FROM golang:1.24-alpine AS builder

# Install templ
RUN go install github.com/a-h/templ/cmd/templ@latest

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Generate templ files
RUN templ generate

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main .

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

COPY --from=builder /app/main .

# Expose port
EXPOSE 9090

CMD ["./main"] 