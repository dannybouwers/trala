### STAGE 1: Build Tailwind CSS ###
FROM node:25.7.0-alpine AS tailwind-builder

WORKDIR /app

# Copy Tailwind configuration and source files
COPY web/html/* web/css/* web/js/* /app/src/

# Install Tailwind CSS and build it
RUN npm install tailwindcss @tailwindcss/cli

# Create a minimal tailwind.css file
RUN npx @tailwindcss/cli -i /app/src/tailwind.src.css -o /app/src/tailwind.css

### STAGE 2: Build Go Application ###
FROM golang:1.26.0-alpine AS builder

# Install build essentials for static compilation
RUN apk add --no-cache build-base

# Accept version as build argument
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_TIME=unknown

WORKDIR /app

# Copy Go project
COPY cmd cmd/
COPY go.mod go.sum ./

# Build the application as a statically linked binary with version info
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags "-X main.version=${VERSION} -X main.commit=${COMMIT} -X main.buildTime=${BUILD_TIME}" -o /server ./cmd/server/


### STAGE 3: Production ###
# Start with a minimal Alpine image
FROM alpine:3.23

# Set a working directory
WORKDIR /app

# Copy the compiled Go binary from the builder stage
COPY --from=builder /server /app/server

# Copy the frontend files into a 'static' directory
COPY --exclude=*.src.css --exclude=html/index.html web /app/static/

# Copy the translations code
COPY translations/* /app/translations/

# Copy the compiled Tailwind CSS from the tailwind-builder stage
COPY --from=tailwind-builder /app/src/tailwind.css /app/static/css/tailwind.css

# Copy the html template into a 'template' directory
COPY web/html/index.html /app/template/index.html

# Expose the port the Go server is listening on
EXPOSE 8080

# Create a directory for optional user-provided configuration
VOLUME /config

# Create a directory for optional user-provided icons
VOLUME /icons

# Install curl for healthcheck
RUN apk add --no-cache curl

# Add healthcheck
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
  CMD curl -f http://localhost:8080/api/health || exit 1

# The command to run the application.
CMD ["/app/server"]
