FROM golang:1.22-alpine AS builder

WORKDIR /app

# First, install build dependencies
RUN apk add --no-cache gcc musl-dev

# Copy go.mod and go.sum first
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the entire project structure
COPY . .

# Build the application (updating the path to your main.go)
RUN CGO_ENABLED=0 GOOS=linux go build -o main ./cmd/server/

# Create a minimal production image
FROM alpine:latest

WORKDIR /app

# Install runtime dependencies if any (e.g., for handling SSL/TLS)
RUN apk --no-cache add ca-certificates

# Copy the binary from builder
COPY --from=builder /app/main .

# Copy necessary directories
COPY --from=builder /app/templates ./templates
COPY --from=builder /app/static ./static

# Set environment variables
ENV CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

EXPOSE 8080

CMD ["./main"]