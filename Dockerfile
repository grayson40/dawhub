FROM golang:1.22-alpine

WORKDIR /app

# Copy go.mod and go.sum first
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the templates directory
COPY templates/ templates/

# Copy the rest of the code
COPY . .

RUN apk add --no-cache gcc musl-dev && \
    go build -o main .

EXPOSE 8080
CMD ["./main"]