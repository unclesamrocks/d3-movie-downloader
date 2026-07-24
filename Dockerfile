# Runtime stage - uses pre-built binary
FROM golang:1.26-alpine
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY bin/d3-movie-downloader .
COPY .env.example ./.env.example
EXPOSE 3000
CMD ["./d3-movie-downloader"]
