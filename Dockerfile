# Build stage
FROM node:18-alpine AS frontend-builder

# Set working directory for frontend
WORKDIR /frontend

# Copy frontend files
COPY web/package*.json ./
RUN npm install

# Copy frontend source and build
COPY web/ ./
RUN npm run build

# Go build stage
FROM golang:1.21-alpine AS backend-builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X main.AppVersion=1.0.0" \
    -o tavily-load \
    ./cmd/tavily-load

# Final stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata curl

# Create non-root user
RUN addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup

# Set working directory
WORKDIR /app

# Copy binary from builder stage
COPY --from=backend-builder /build/tavily-load .

# Copy frontend build from frontend-builder
COPY --from=frontend-builder /frontend/out ./web/out

# Copy configuration files
COPY .env.example .env
COPY keys.txt.example keys.txt

# Create logs directory
RUN mkdir -p logs && \
    chown -R appuser:appgroup /app

# Switch to non-root user
USER appuser

# Expose port
EXPOSE 3000

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:3000/health || exit 1

# Run the application
CMD ["./tavily-load"]