# Build stage: use Go 1.24 to satisfy "go mod requires go >= 1.24"
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy go.mod and go.sum, then download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy all source files and build the binary
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o gateway ./cmd/gateway

# Final stage: minimal runtime image
FROM alpine:3.18

# Install CA certificates for HTTPS support
RUN apk add --no-cache ca-certificates

WORKDIR /root/
# Copy compiled binary and configuration file
COPY --from=builder /app/gateway .
COPY config.yml .

EXPOSE 80

CMD ["./gateway"]
