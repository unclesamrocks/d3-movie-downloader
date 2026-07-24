# Builder stage
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 go build -o /d3-movie-downloader ./cmd/bot

# Runtime stage
FROM golang:1.23-alpine
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /d3-movie-downloader .
COPY .env.example ./.env.example
EXPOSE 3000
CMD ["./d3-movie-downloader"]
