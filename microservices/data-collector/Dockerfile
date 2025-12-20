FROM golang:1.25.1-alpine AS builder
RUN apk add --no-cache git build-base
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -tags musl -o data-collector ./microservices/data-collector/cmd/main.go
FROM alpine:latest
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /app/data-collector .
EXPOSE 8080
CMD ["./data-collector"]