# Build stage
FROM --platform=linux/arm64 public.ecr.aws/docker/library/golang:1.23-alpine AS builder

# Set working directory
WORKDIR /app

# Install necessary build tools
RUN apk add --no-cache git ca-certificates tzdata

# Copy go.mod and go.sum
COPY go.mod go.sum ./

# Download Go dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application for ARM64 Linux
RUN CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -a -installsuffix cgo -o main .

# Production stage
FROM --platform=linux/arm64 public.ecr.aws/docker/library/alpine:latest

# Set working directory
WORKDIR /home/appuser/

# Install necessary runtime packages
RUN apk --no-cache add ca-certificates tzdata && \
    addgroup -g 1001 appgroup && \
    adduser -D -s /bin/sh -u 1001 -G appgroup appuser

# Copy the compiled binary
COPY --from=builder /app/main .

# Change ownership
RUN chown -R appuser:appgroup /home/appuser/

# Use non-root user
USER appuser

# Expose application port
EXPOSE 8080

# Healthcheck
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Start the application
CMD ["./main"]
