# Build Stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install git for fetching dependencies
RUN apk add --no-cache git

# Copy dependency files
COPY go.mod go.sum ./

# Since we are in a monorepo-like structure locally, we might need special handling.
# But assuming standard build context:
RUN go mod download

# Copy source code
COPY . .

# Build binary
RUN CGO_ENABLED=0 GOOS=linux go build -o polygate-server ./cmd/server

# Runtime Stage
FROM alpine:latest

WORKDIR /app

# Install CA certificates for HTTPS
RUN apk --no-cache add ca-certificates

# Copy binary from builder
COPY --from=builder /app/polygate-server .
COPY --from=builder /app/config.yaml.example ./config.yaml

# Expose port
EXPOSE 8080

# Run
CMD ["./polygate-server"]
