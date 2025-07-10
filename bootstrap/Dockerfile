# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /workspace

# Copy the entire workspace structure
COPY shared/ ./shared/
COPY bootstrap/ ./bootstrap/

# Set working directory to bootstrap for build
WORKDIR /workspace/bootstrap

RUN go mod download

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main .

FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

COPY --from=builder /workspace/bootstrap/main .

EXPOSE 8080

CMD ["./main"] 