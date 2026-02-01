# Build Stage
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Build
COPY . .
RUN go build -o /bin/polygate ./cmd/server
RUN go build -o /bin/inspector ./cmd/inspector

# Run Stage
FROM alpine:latest

WORKDIR /app

# Install CA certificates for HTTPS
RUN apk --no-cache add ca-certificates tzdata

COPY --from=builder /bin/polygate .
COPY --from=builder /bin/inspector .
COPY config.yaml.example ./config.yaml

# Expose API and Metrics ports
EXPOSE 8080

CMD ["./polygate"]