FROM golang:1.23-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
COPY main.go config.yml ./

RUN go mod tidy

RUN CGO_ENABLED=0 GOOS=linux go build -o gateway .

FROM alpine:latest

RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY --from=builder /app/gateway .

COPY --from=builder /app/config.yml .

EXPOSE 80

ENTRYPOINT ["./gateway", "--config", "config.yml"]
