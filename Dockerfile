FROM golang:1.21-alpine

WORKDIR /app

RUN apk add --no-cache git  # required for go mod download

# Copy go.mod and go.sum files to install dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source code
COPY . .

# Build the Go application
RUN CGO_ENABLED=0 GOOS=linux go build -o gocluster-manager ./cmd/gocluster-manager/main.go

# Expose necessary ports
EXPOSE 8080 7946

# Set the entrypoint
ENTRYPOINT ["/app/gocluster-manager", "-c", "/app/cluster.yaml"]
