
FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o go_gateway .

FROM alpine:latest

RUN apk add --no-cache ca-certificates

WORKDIR /root

COPY --from=builder /app/go_gateway .

COPY --from=builder /app/config.yml .

EXPOSE 80

CMD ["./go_gateway", "--config", "config.yml"]
