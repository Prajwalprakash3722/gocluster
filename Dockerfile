# Build stage
FROM ubuntu:22.04 AS builder

# Set non-interactive installation
ENV DEBIAN_FRONTEND=noninteractive

# Install Go and essential tools
RUN apt-get install -y \
  golang-go \
  git \
  make \
  && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Copy go mod files
COPY go.mod ./
COPY go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o agent cmd/agent/main.go

# Final stage
FROM ubuntu:22.04

WORKDIR /app
COPY --from=builder /app/agent .

ENTRYPOINT ["/app/agent"]
