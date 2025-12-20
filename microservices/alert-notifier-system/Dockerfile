FROM golang:1.25.1-alpine AS builder
RUN apk add --no-cache git build-base
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -tags musl -o alert-notifier-system ./microservices/alert-notifier-system/cmd/main.go
FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/alert-notifier-system .
CMD ["./alert-notifier-system"]