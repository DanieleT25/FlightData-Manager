FROM golang:1.25.1-alpine AS builder
RUN apk add --no-cache git
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o data-collector ./microservices/data-collector/cmd/main.go
FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/data-collector .
EXPOSE 8080
CMD ["./data-collector"]
