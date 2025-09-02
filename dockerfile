### STAGE 1: Build Go Application ###
FROM golang:1.25-alpine AS builder

# Install build essentials for static compilation
RUN apk add --no-cache build-base

WORKDIR /app

# Copy Go module files and download dependencies
COPY server/go.mod server/go.sum ./
RUN go mod download

# Copy the source code
COPY server/main.go .

# Build the application as a statically linked binary.
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /server .


### STAGE 2: Production ###
# Start with a minimal Alpine image
FROM alpine:3.22

# Set a working directory
WORKDIR /app

# Copy the compiled Go binary from the builder stage
COPY --from=builder /server /app/server

# Copy the static frontend files into a 'static' directory
COPY static/* /app/static/

# Copy the html template into a 'template' directory
COPY index.html /app/template/index.html

# Expose the port the Go server is listening on
EXPOSE 8080

# Create a directory for optional user-provided configuration
VOLUME /config

# The command to run the application.
CMD ["/app/server"]
