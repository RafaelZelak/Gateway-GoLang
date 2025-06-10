# build stage
FROM golang:1.24-alpine AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o gateway ./cmd/gateway

# runtime stage
FROM alpine:3.18

RUN apk add --no-cache \
    ca-certificates \
    bash \
    python3 \
    py3-pip \
    go

WORKDIR /root

# copia binário e configs
COPY --from=builder /app/gateway .
COPY --from=builder /app/config.yml .
COPY --from=builder /app/jobs.yml .
# copia todos os jobs (cada pasta pode ter seu requirements.txt)
COPY --from=builder /app/jobs ./jobs

# instala deps Python de TODOS os jobs que tenham requirements.txt
RUN find ./jobs -type f -name requirements.txt -exec pip3 install --no-cache-dir -r {} \;

EXPOSE 80
CMD ["./gateway"]
