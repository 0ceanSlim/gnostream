# Dockerfile
FROM golang:1.22.4-alpine AS builder

# Update and install FFmpeg and other dependencies
RUN apk update && apk upgrade && apk add --no-cache ffmpeg git

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o stream-server ./cmd/server

FROM alpine:3.20.2

# Update and install FFmpeg
RUN apk update && apk upgrade && apk add --no-cache ffmpeg ca-certificates

WORKDIR /app

# Copy binary and static files
COPY --from=builder /app/stream-server .
COPY --from=builder /app/web ./web
COPY --from=builder /app/configs ./configs

# Create directories
RUN mkdir -p web/live web/live/past-streams

# Expose port
EXPOSE 8080

# Run the server
CMD ["./stream-server"]
