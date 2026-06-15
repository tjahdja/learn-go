# --- STAGE 1: Build the binary ---
FROM golang:1.26-alpine AS builder

# Set the working directory inside the container
WORKDIR /app

# Copy the dependency files first (enables Docker caching for faster builds)
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of your application source code
COPY . .

# Compile your Go application into a single, static binary named 'server'
RUN CGO_ENABLED=0 GOOS=linux go build -o /server main.go

# --- STAGE 2: Run the binary ---
FROM alpine:latest

WORKDIR /

# Copy the compiled binary from Stage 1
COPY --from=builder /server /server

COPY --from=builder /app/migrations /migrations

# Expose port 8080 to the outside world
EXPOSE 8080

# Run your compiled binary when the container starts
CMD ["/server"]