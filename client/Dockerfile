FROM golang:1.24-alpine AS builder

# Nur templ installieren (kein gcc/musl-dev mehr nötig!)
RUN go install github.com/a-h/templ/cmd/templ@latest

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN templ generate

# Build OHNE CGO (modernc.org/sqlite benötigt es nicht!)
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -trimpath -o main .

# Finale Stage (keine sqlite-Runtime-Deps mehr nötig)
FROM alpine:latest
WORKDIR /root/

# ➕ curl hinzufügen
RUN apk add --no-cache curl

COPY --from=builder /app/main .
EXPOSE 9090
CMD ["./main"]